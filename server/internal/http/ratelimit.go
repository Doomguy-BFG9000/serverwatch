package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter is a small in-memory token-bucket limiter keyed by client IP.
//
// It exists to blunt accidental or abusive request floods on a single-host
// Community Edition deployment. It is deliberately not a distributed limiter:
// state is per process and resets on restart, which is acceptable for the
// single-node target. The middleware wraps only the API; the static dashboard
// assets are not counted.
type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	rps        float64
	burst      float64
	trustProxy bool
	now        func() time.Time
	lastGC     time.Time
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// NewRateLimiter builds a limiter allowing rps requests/second per client with
// the given burst. Non-positive values disable limiting (returns nil).
func NewRateLimiter(rps float64, burst int, trustProxy bool) *RateLimiter {
	if rps <= 0 || burst <= 0 {
		return nil
	}
	return &RateLimiter{
		buckets:    make(map[string]*bucket),
		rps:        rps,
		burst:      float64(burst),
		trustProxy: trustProxy,
		now:        time.Now,
		lastGC:     time.Now(),
	}
}

// Middleware returns a handler that rejects over-limit clients with 429. A nil
// limiter is a pass-through.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	if l == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if !l.allow(l.clientIP(r)) {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "rate_limited"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.gcLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, lastSeen: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	b.lastSeen = now
	b.tokens += elapsed * l.rps
	if b.tokens > l.burst {
		b.tokens = l.burst
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// gcLocked drops buckets that have been idle long enough to have refilled
// completely, bounding memory for long-running processes.
func (l *RateLimiter) gcLocked(now time.Time) {
	if now.Sub(l.lastGC) < time.Minute {
		return
	}
	l.lastGC = now
	idle := now.Add(-time.Minute)
	for key, b := range l.buckets {
		if b.lastSeen.Before(idle) {
			delete(l.buckets, key)
		}
	}
}

// clientIP returns the request's client address. When the deployment is behind
// a trusted reverse proxy (the documented Nginx setup), the first X-Forwarded-For
// entry is used. The immediate peer is used as a fallback and as the key when
// proxy trust is disabled.
func (l *RateLimiter) clientIP(r *http.Request) string {
	if l.trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if first != "" {
				return first
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
