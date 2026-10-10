package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/repository"
	"github.com/serverwatch/serverwatch/server/internal/service"
)

// sessionCookieName is the single session cookie used by the dashboard.
const sessionCookieName = "serverwatch_session"

// AuthStore is the persistence surface the handler needs. Keeping it as an
// interface lets the HTTP tests run without PostgreSQL.
type AuthStore interface {
	CountUsers(context.Context) (int, error)
	CreateFirstUser(context.Context, string, string, string) (service.User, bool, error)
	FindUserByEmail(context.Context, string) (service.User, string, error)
	CreateSession(context.Context, string, int64, time.Time) error
	FindUserBySession(context.Context, string) (service.User, error)
	DeleteSession(context.Context, string) error
	DeleteExpiredSessions(context.Context) error
}

type AuthHandler struct {
	Store        AuthStore
	AuthEnabled  bool
	SessionTTL   time.Duration
	CookieSecure bool
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.CookieSecure,
		MaxAge:   int(h.SessionTTL.Seconds()),
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.CookieSecure,
		MaxAge:   -1,
	})
}

// currentUser resolves the caller's session cookie to a user, if any.
func (h *AuthHandler) currentUser(r *http.Request) (service.User, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return service.User{}, false
	}
	user, err := h.Store.FindUserBySession(
		r.Context(),
		service.HashSessionToken(cookie.Value),
	)
	if err != nil {
		return service.User{}, false
	}
	return user, true
}

// Status reports whether authentication is enabled, whether first-run setup is
// still required, and, when a valid session is presented, the current user.
func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	status := service.AuthStatus{AuthEnabled: h.AuthEnabled}

	if h.AuthEnabled {
		count, err := h.Store.CountUsers(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to read auth status"})
			return
		}
		status.SetupRequired = count == 0
	}

	if user, ok := h.currentUser(r); ok {
		status.Authenticated = true
		status.User = &user
	}

	writeJSON(w, http.StatusOK, status)
}

// Setup creates the first administrator account. It only succeeds when
// authentication is enabled and no account exists yet, so it can never be used
// to add extra users or take over an initialized installation.
func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	if !h.AuthEnabled {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
		return
	}
	if _, ok := h.currentUser(r); ok {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "already_authenticated"})
		return
	}

	var input struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}

	email, err := service.ValidateEmail(input.Email)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	displayName, err := service.ValidateDisplayName(input.DisplayName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := service.ValidatePassword(input.Password); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	hash, err := service.HashPassword(input.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to set up account"})
		return
	}

	user, created, err := h.Store.CreateFirstUser(r.Context(), email, displayName, hash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to set up account"})
		return
	}
	if !created {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "already_initialized"})
		return
	}

	if err := h.startSession(w, r, user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to start session"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

// Login verifies credentials and starts a session. Failures use a single
// generic message so the endpoint does not reveal whether an email exists.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.AuthEnabled {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
		return
	}

	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}

	email := service.NormalizeEmail(input.Email)
	user, passwordHash, err := h.Store.FindUserByEmail(r.Context(), email)
	if err != nil || !service.VerifyPassword(passwordHash, input.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_credentials"})
		return
	}

	_ = h.Store.DeleteExpiredSessions(r.Context())

	if err := h.startSession(w, r, user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to start session"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

// Logout deletes the current session, if any. It is idempotent.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		_ = h.Store.DeleteSession(r.Context(), service.HashSessionToken(cookie.Value))
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the current user or 401 when no valid session is presented.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

// startSession creates a session row and sets the session cookie.
func (h *AuthHandler) startSession(w http.ResponseWriter, r *http.Request, user service.User) error {
	token, err := service.NewSessionToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(h.SessionTTL)
	if err := h.Store.CreateSession(r.Context(), service.HashSessionToken(token), user.ID, expiresAt); err != nil {
		return err
	}
	h.setSessionCookie(w, token)
	return nil
}

// Ensure the repository satisfies the handler interface at compile time.
var _ AuthStore = (*repository.AuthRepository)(nil)
