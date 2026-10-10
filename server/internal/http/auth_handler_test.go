package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

// fakeAuthStore is an in-memory AuthStore for handler tests. It exercises the
// HTTP behavior without PostgreSQL while preserving real hashing and session
// semantics (the handler and service do the crypto).
type fakeAuthStore struct {
	users    map[string]fakeAccount
	sessions map[string]int64
	nextID   int64
}

type fakeAccount struct {
	user service.User
	hash string
}

func newFakeAuthStore() *fakeAuthStore {
	return &fakeAuthStore{
		users:    map[string]fakeAccount{},
		sessions: map[string]int64{},
	}
}

func (s *fakeAuthStore) CountUsers(context.Context) (int, error) {
	return len(s.users), nil
}

func (s *fakeAuthStore) CreateFirstUser(_ context.Context, email, displayName, hash string) (service.User, bool, error) {
	if len(s.users) > 0 {
		return service.User{}, false, nil
	}
	s.nextID++
	user := service.User{ID: s.nextID, Email: email, DisplayName: displayName, IsAdmin: true, CreatedAt: time.Now()}
	s.users[email] = fakeAccount{user: user, hash: hash}
	return user, true, nil
}

func (s *fakeAuthStore) FindUserByEmail(_ context.Context, email string) (service.User, string, error) {
	account, ok := s.users[email]
	if !ok {
		return service.User{}, "", errNotFound{}
	}
	return account.user, account.hash, nil
}

func (s *fakeAuthStore) CreateSession(_ context.Context, tokenHash string, userID int64, _ time.Time) error {
	s.sessions[tokenHash] = userID
	return nil
}

func (s *fakeAuthStore) FindUserBySession(_ context.Context, tokenHash string) (service.User, error) {
	userID, ok := s.sessions[tokenHash]
	if !ok {
		return service.User{}, errNotFound{}
	}
	for _, account := range s.users {
		if account.user.ID == userID {
			return account.user, nil
		}
	}
	return service.User{}, errNotFound{}
}

func (s *fakeAuthStore) DeleteSession(_ context.Context, tokenHash string) error {
	delete(s.sessions, tokenHash)
	return nil
}

func (s *fakeAuthStore) DeleteExpiredSessions(context.Context) error { return nil }

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

func newAuthTestMux(handler *AuthHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /api/v1/auth/status", handler.Status)
	mux.HandleFunc("POST /api/v1/auth/setup", handler.Setup)
	mux.HandleFunc("POST /api/v1/auth/login", handler.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", handler.Logout)
	mux.HandleFunc("GET /api/v1/auth/me", handler.Me)
	mux.HandleFunc("GET /api/v1/monitors/status", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	return mux
}

func enabledHandler(store AuthStore) *AuthHandler {
	return &AuthHandler{Store: store, AuthEnabled: true, SessionTTL: time.Hour}
}

func postJSON(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func sessionCookie(recorder *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie
		}
	}
	return nil
}

func TestAuthStatusSetupRequired(t *testing.T) {
	handler := enabledHandler(newFakeAuthStore())
	recorder := httptest.NewRecorder()

	handler.Status(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil))

	var status service.AuthStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.AuthEnabled || !status.SetupRequired || status.Authenticated {
		t.Fatalf("unexpected fresh status: %+v", status)
	}
}

func TestAuthSetupCreatesFirstAdminOnly(t *testing.T) {
	handler := enabledHandler(newFakeAuthStore())

	first := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/setup", `{"email":"admin@example.com","password":"supersecret","display_name":"Admin"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first setup expected 201, got %d (%s)", first.Code, first.Body.String())
	}
	if sessionCookie(first) == nil {
		t.Fatal("setup should set a session cookie")
	}

	second := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/setup", `{"email":"other@example.com","password":"supersecret","display_name":"Other"}`)
	if second.Code != http.StatusConflict {
		t.Fatalf("second setup expected 409, got %d", second.Code)
	}
}

func TestAuthSetupRejectedWhenDisabled(t *testing.T) {
	handler := &AuthHandler{Store: newFakeAuthStore(), SessionTTL: time.Hour}
	recorder := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/setup", `{"email":"admin@example.com","password":"supersecret"}`)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("setup when disabled expected 404, got %d", recorder.Code)
	}
}

func TestAuthSetupValidatesInput(t *testing.T) {
	handler := enabledHandler(newFakeAuthStore())
	recorder := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/setup", `{"email":"bad","password":"short"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid setup expected 400, got %d", recorder.Code)
	}
}

func TestAuthLoginFlow(t *testing.T) {
	handler := enabledHandler(newFakeAuthStore())
	setup := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/setup", `{"email":"admin@example.com","password":"supersecret"}`)
	if setup.Code != http.StatusCreated {
		t.Fatalf("setup expected 201, got %d", setup.Code)
	}

	bad := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/login", `{"email":"admin@example.com","password":"wrong"}`)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password expected 401, got %d", bad.Code)
	}

	sameMessage := bad.Body.String()
	unknown := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/login", `{"email":"nobody@example.com","password":"whatever"}`)
	if unknown.Code != http.StatusUnauthorized || unknown.Body.String() != sameMessage {
		t.Fatalf("unknown email must produce the same 401 body, got %d %s", unknown.Code, unknown.Body.String())
	}

	good := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/login", `{"email":"Admin@Example.com","password":"supersecret"}`)
	if good.Code != http.StatusOK {
		t.Fatalf("valid login expected 200, got %d (%s)", good.Code, good.Body.String())
	}
	if sessionCookie(good) == nil {
		t.Fatal("successful login should set a session cookie")
	}
}

func TestAuthMeAndLogout(t *testing.T) {
	handler := enabledHandler(newFakeAuthStore())
	setup := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/setup", `{"email":"admin@example.com","password":"supersecret"}`)
	cookie := sessionCookie(setup)
	if cookie == nil {
		t.Fatal("expected session cookie")
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meRequest.AddCookie(cookie)
	meRecorder := httptest.NewRecorder()
	handler.Me(meRecorder, meRequest)
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("me with session expected 200, got %d", meRecorder.Code)
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	logoutRecorder := httptest.NewRecorder()
	handler.Logout(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusNoContent {
		t.Fatalf("logout expected 204, got %d", logoutRecorder.Code)
	}

	afterLogout := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	afterRecorder := httptest.NewRecorder()
	handler.Me(afterRecorder, afterLogout)
	if afterRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("me without session expected 401, got %d", afterRecorder.Code)
	}
}

func TestAuthMiddlewareProtectsAPIButNotAuthEntryPoints(t *testing.T) {
	store := newFakeAuthStore()
	handler := enabledHandler(store)
	setup := postJSON(t, newAuthTestMux(handler), "/api/v1/auth/setup", `{"email":"admin@example.com","password":"supersecret"}`)
	cookie := sessionCookie(setup)

	protected := NewAuthMiddleware(true, "", store, newAuthTestMux(handler))

	// Unauthenticated API access is rejected.
	recorder := httptest.NewRecorder()
	protected.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("protected API without session expected 401, got %d", recorder.Code)
	}

	// Auth entry points and health stay open.
	for _, path := range []string{"/api/v1/auth/status", "/health"} {
		open := httptest.NewRecorder()
		protected.ServeHTTP(open, httptest.NewRequest(http.MethodGet, path, nil))
		if open.Code != http.StatusOK {
			t.Fatalf("%s should stay open, got %d", path, open.Code)
		}
	}

	// A valid session cookie grants access.
	authedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil)
	authedRequest.AddCookie(cookie)
	authed := httptest.NewRecorder()
	protected.ServeHTTP(authed, authedRequest)
	if authed.Code != http.StatusOK {
		t.Fatalf("protected API with session expected 200, got %d", authed.Code)
	}
}

func TestAuthMiddlewareTokenStillWorks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/monitors/status", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	protected := NewAuthMiddleware(true, "shared-secret", newFakeAuthStore(), mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil)
	request.Header.Set("Authorization", "Bearer shared-secret")
	recorder := httptest.NewRecorder()
	protected.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("bearer token should grant access, got %d", recorder.Code)
	}
}
