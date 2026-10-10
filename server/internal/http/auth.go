package httpapi

// Authentication middleware (opt-in).
//
// ServerWatch Community Edition supports two independent, opt-in mechanisms:
//
//   * A shared bearer token (SERVERWATCH_API_TOKEN) for scripts and the
//     Phase 0.48 token flow.
//   * Account sessions (SERVERWATCH_AUTH_ENABLED=true) using an HttpOnly
//     session cookie created by the login endpoint.
//
// When neither is configured the middleware is a pass-through and the historic
// open behavior is preserved, so enabling authentication never breaks an
// existing installation. Health endpoints and the auth entry points stay open
// so probes and login keep working.

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

// NewAPITokenMiddleware enforces a small shared bearer token on every path
// under /api/. It is the token-only form of NewAuthMiddleware and is kept for
// callers that only need the shared-secret behavior.
func NewAPITokenMiddleware(token string, next http.Handler) http.Handler {
	return NewAuthMiddleware(false, token, nil, next)
}

// NewAuthMiddleware returns a middleware that protects /api/ routes with either
// a valid bearer token or a valid session cookie, depending on what is enabled.
// Authentication endpoints and health probes are always reachable.
func NewAuthMiddleware(enabled bool, apiToken string, store AuthStore, next http.Handler) http.Handler {
	if !enabled && apiToken == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		if isAuthPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if apiToken != "" && validBearerToken(r.Header.Get("Authorization"), apiToken) {
			next.ServeHTTP(w, r)
			return
		}

		if enabled && store != nil {
			if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
				if _, err := store.FindUserBySession(r.Context(), service.HashSessionToken(cookie.Value)); err == nil {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		w.Header().Set("WWW-Authenticate", `Bearer realm="serverwatch"`)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
	})
}

// isAuthPublicPath lists API routes that must remain reachable without a
// session so the dashboard can discover the auth model and the user can sign
// in, sign up the first administrator, or sign out.
func isAuthPublicPath(path string) bool {
	switch path {
	case "/api/v1/auth/status", "/api/v1/auth/login", "/api/v1/auth/setup", "/api/v1/auth/logout":
		return true
	default:
		return false
	}
}

// validBearerToken reports whether the Authorization header carries exactly the
// configured bearer token. The comparison is constant-time and also guards the
// scheme so non-bearer headers never authenticate.
func validBearerToken(header, token string) bool {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return false
	}
	presented := header[len(prefix):]
	return subtle.ConstantTimeCompare([]byte(presented), []byte(token)) == 1
}
