package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersSetOnEveryResponse(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := SecurityHeaders(next)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil))

	expected := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
	}
	for name, want := range expected {
		if got := recorder.Header().Get(name); got != want {
			t.Fatalf("header %s = %q, want %q", name, got, want)
		}
	}
	if recorder.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("Content-Security-Policy must be set")
	}
}

func TestSecurityHeadersHSTSOnlyOverTLS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Plain HTTP: no HSTS.
	plain := httptest.NewRecorder()
	SecurityHeaders(next).ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/", nil))
	if plain.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS must not be set over plain HTTP")
	}

	// Forwarded HTTPS (Nginx terminating TLS): HSTS present.
	proxied := httptest.NewRequest(http.MethodGet, "/", nil)
	proxied.Header.Set("X-Forwarded-Proto", "https")
	proxiedRecorder := httptest.NewRecorder()
	SecurityHeaders(next).ServeHTTP(proxiedRecorder, proxied)
	if proxiedRecorder.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("HSTS must be set when X-Forwarded-Proto is https")
	}
}
