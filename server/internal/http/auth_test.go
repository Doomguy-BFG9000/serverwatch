package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTokenTestServer(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/monitors/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return NewAPITokenMiddleware(token, mux)
}

func doRequest(t *testing.T, handler http.Handler, path, authorization string) int {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Code
}

func TestAPITokenEmptyIsPassThrough(t *testing.T) {
	handler := newTokenTestServer("")

	if status := doRequest(t, handler, "/api/v1/monitors/status", ""); status != http.StatusOK {
		t.Fatalf("expected 200 when no token configured, got %d", status)
	}
}

func TestAPITokenRequiresBearer(t *testing.T) {
	handler := newTokenTestServer("s3cr3t-token")

	cases := []struct {
		name          string
		authorization string
		want          int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"wrong token", "Bearer nope", http.StatusUnauthorized},
		{"wrong scheme", "Basic s3cr3t-token", http.StatusUnauthorized},
		{"bare token no scheme", "s3cr3t-token", http.StatusUnauthorized},
		{"correct token", "Bearer s3cr3t-token", http.StatusOK},
		{"correct token different case scheme", "bearer s3cr3t-token", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := doRequest(t, handler, "/api/v1/monitors/status", tc.authorization); status != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, status)
			}
		})
	}
}

func TestAPITokenLeavesHealthOpen(t *testing.T) {
	handler := newTokenTestServer("s3cr3t-token")

	for _, path := range []string{"/health", "/ready"} {
		if status := doRequest(t, handler, path, ""); status != http.StatusOK {
			t.Fatalf("expected %s to stay open, got %d", path, status)
		}
	}
}

func TestValidBearerToken(t *testing.T) {
	if validBearerToken("Bearer token", "") {
		t.Fatal("empty configured token must never authenticate")
	}
	if !validBearerToken("Bearer token", "token") {
		t.Fatal("exact bearer token should authenticate")
	}
	if validBearerToken("Bearer token-x", "token") {
		t.Fatal("longer presented token must not authenticate")
	}
}
