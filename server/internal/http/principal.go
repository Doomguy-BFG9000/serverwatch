package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

// principal describes who made a request once the auth middleware has
// authenticated it. It is attached to the request context so downstream
// middleware (the audit log) can attribute actions without re-parsing cookies.
type principal struct {
	User     *service.User
	ViaToken bool
}

type principalContextKey struct{}

func withPrincipal(ctx context.Context, p principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, p)
}

func principalFromContext(ctx context.Context) (principal, bool) {
	p, ok := ctx.Value(principalContextKey{}).(principal)
	return p, ok
}

// UserFromContext returns the authenticated session user, if any.
func UserFromContext(ctx context.Context) (service.User, bool) {
	p, ok := principalFromContext(ctx)
	if !ok || p.User == nil {
		return service.User{}, false
	}
	return *p.User, true
}

// clientIP returns the best-known client address for logging and rate limiting.
// The documented deployment puts Nginx in front of the server, which forwards
// the original address in X-Forwarded-For; the immediate peer is the fallback.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if first != "" {
			return first
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
