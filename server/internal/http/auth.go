package httpapi

// API token authentication (opt-in).
//
// ServerWatch Community Edition can require a bearer token on its API surface
// when the operator sets SERVERWATCH_API_TOKEN. When the variable is empty the
// middleware is a pass-through and the historic open behavior is preserved, so
// enabling this feature can never break an existing installation.
//
// Health endpoints are intentionally exempt so container and systemd
// liveness/readiness probes keep working after a token is configured.

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// NewAPITokenMiddleware returns a middleware that enforces a bearer token on
// every path under /api/. The token is compared with crypto/subtle to avoid
// leaking the value through response-timing differences. When token is empty
// the original handler is returned unchanged.
func NewAPITokenMiddleware(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		if !validBearerToken(r.Header.Get("Authorization"), token) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="serverwatch"`)
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": "unauthorized",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
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
