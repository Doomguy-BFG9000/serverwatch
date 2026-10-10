package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimiterAllowsBurstThenBlocks(t *testing.T) {
	limiter := NewRateLimiter(1, 2, false)
	handler := limiter.Middleware(okHandler())

	request := func() int {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil)
		r.RemoteAddr = "203.0.113.7:5555"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Code
	}

	if code := request(); code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", code)
	}
	if code := request(); code != http.StatusOK {
		t.Fatalf("second request (burst) expected 200, got %d", code)
	}
	if code := request(); code != http.StatusTooManyRequests {
		t.Fatalf("third request expected 429, got %d", code)
	}
}

func TestRateLimiterRefills(t *testing.T) {
	limiter := NewRateLimiter(10, 1, false)
	fake := time.Now()
	limiter.now = func() time.Time { return fake }
	handler := limiter.Middleware(okHandler())

	request := func() int {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil)
		r.RemoteAddr = "203.0.113.9:5555"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Code
	}

	if code := request(); code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", code)
	}
	if code := request(); code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d", code)
	}

	fake = fake.Add(200 * time.Millisecond) // 10 rps * 0.2s = 2 tokens
	if code := request(); code != http.StatusOK {
		t.Fatalf("request after refill expected 200, got %d", code)
	}
}

func TestRateLimiterIgnoresStaticAssets(t *testing.T) {
	limiter := NewRateLimiter(1, 1, false)
	handler := limiter.Middleware(okHandler())

	for i := 0; i < 5; i++ {
		r := httptest.NewRequest(http.MethodGet, "/index.html", nil)
		r.RemoteAddr = "203.0.113.11:5555"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		if recorder.Code != http.StatusOK {
			t.Fatalf("static asset request %d expected 200, got %d", i, recorder.Code)
		}
	}
}

func TestRateLimiterSeparatesClients(t *testing.T) {
	limiter := NewRateLimiter(1, 1, false)
	handler := limiter.Middleware(okHandler())

	call := func(addr string) int {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil)
		r.RemoteAddr = addr
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Code
	}

	if code := call("198.51.100.1:1111"); code != http.StatusOK {
		t.Fatalf("client A first expected 200, got %d", code)
	}
	if code := call("198.51.100.2:2222"); code != http.StatusOK {
		t.Fatalf("client B first expected 200, got %d", code)
	}
	if code := call("198.51.100.1:1111"); code != http.StatusTooManyRequests {
		t.Fatalf("client A second expected 429, got %d", code)
	}
}

func TestRateLimiterUsesForwardedForWhenTrusted(t *testing.T) {
	limiter := NewRateLimiter(1, 1, true)
	handler := limiter.Middleware(okHandler())

	call := func(forwarded string) int {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil)
		r.RemoteAddr = "127.0.0.1:4444"
		r.Header.Set("X-Forwarded-For", forwarded)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Code
	}

	if code := call("203.0.113.50"); code != http.StatusOK {
		t.Fatalf("first forwarded client expected 200, got %d", code)
	}
	if code := call("203.0.113.51"); code != http.StatusOK {
		t.Fatalf("different forwarded client expected 200, got %d", code)
	}
	if code := call("203.0.113.50"); code != http.StatusTooManyRequests {
		t.Fatalf("repeat forwarded client expected 429, got %d", code)
	}
}

func TestNilRateLimiterIsPassThrough(t *testing.T) {
	var limiter *RateLimiter
	handler := limiter.Middleware(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, r)
	if recorder.Code != http.StatusOK {
		t.Fatalf("nil limiter expected pass-through 200, got %d", recorder.Code)
	}
}
