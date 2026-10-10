package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsMiddlewareCountsByStatusClass(t *testing.T) {
	metrics := NewMetrics()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/missing") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := metrics.Middleware(inner)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/monitors", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))

	recorder := httptest.NewRecorder()
	metrics.Handler(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()

	if !strings.Contains(body, `serverwatch_http_requests_total{method="GET",status_class="2xx"} 1`) {
		t.Fatalf("2xx counter missing or wrong: %s", body)
	}
	if !strings.Contains(body, `serverwatch_http_requests_total{method="GET",status_class="4xx"} 1`) {
		t.Fatalf("4xx counter missing or wrong: %s", body)
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("unexpected content type: %s", recorder.Header().Get("Content-Type"))
	}
}

func TestMetricsHandlerEmitsGauges(t *testing.T) {
	metrics := NewMetrics()
	metrics.SetGauge("serverwatch_build_info", 1)

	recorder := httptest.NewRecorder()
	metrics.Handler(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()

	if !strings.Contains(body, "# TYPE serverwatch_build_info gauge") {
		t.Fatalf("gauge type line missing: %s", body)
	}
	if !strings.Contains(body, "serverwatch_build_info 1") {
		t.Fatalf("gauge value missing: %s", body)
	}
}

func TestStatusClass(t *testing.T) {
	cases := map[int]string{
		200: "2xx",
		204: "2xx",
		301: "3xx",
		400: "4xx",
		404: "4xx",
		500: "5xx",
		503: "5xx",
	}
	for status, want := range cases {
		if got := statusClass(status); got != want {
			t.Fatalf("statusClass(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestNilMetricsIsPassThrough(t *testing.T) {
	var metrics *Metrics
	handler := metrics.Middleware(okHandler())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/monitors", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("nil metrics expected pass-through, got %d", recorder.Code)
	}
}
