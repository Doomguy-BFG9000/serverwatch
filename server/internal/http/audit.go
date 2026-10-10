package httpapi

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// AuditLog records mutating API calls (POST/PUT/PATCH/DELETE) to the process
// log. It captures who acted (the authenticated user or "anonymous"/"token"),
// what they did (method and path), the outcome status, and the request
// duration. Reads are not audited to keep the signal high.
//
// This is a deliberately lightweight, append-only trail suitable for a
// single-host Community Edition deployment. It is not a tamper-evident or
// queryable audit store; operators who need that should ship the logs to a
// central system.
type AuditLog struct {
	Logger *log.Logger
}

// Middleware logs a single line for each mutating request after it completes.
func (a *AuditLog) Middleware(next http.Handler) http.Handler {
	if a == nil || a.Logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isMutating(r.Method) || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		actor, userID := a.actor(r)
		a.Logger.Printf(
			"audit method=%s path=%s status=%d actor=%s user_id=%s duration_ms=%d remote=%s",
			r.Method,
			r.URL.Path,
			recorder.status,
			actor,
			userID,
			time.Since(started).Milliseconds(),
			clientIP(r),
		)
	})
}

// actor identifies the caller for the audit entry from the auth middleware's
// principal: a session user, the shared API token, or an anonymous caller.
func (a *AuditLog) actor(r *http.Request) (string, string) {
	if p, ok := principalFromContext(r.Context()); ok {
		if p.User != nil {
			return "user", itoa(p.User.ID)
		}
		if p.ViaToken {
			return "token", "-"
		}
	}
	return "anonymous", "-"
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// statusRecorder captures the response status for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(status int) {
	if !s.wrote {
		s.status = status
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.wrote = true
	return s.ResponseWriter.Write(b)
}
