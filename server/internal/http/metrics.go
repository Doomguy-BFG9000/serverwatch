package httpapi

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Metrics is a minimal Prometheus text-format exposition endpoint.
//
// It deliberately avoids a third-party metrics dependency: the Community
// Edition only needs a handful of counters and gauges, and the text exposition
// format is simple and stable. The endpoint is opt-in (SERVERWATCH_METRICS_ENABLED)
// and mounted at /metrics outside the /api/ auth gate so a Prometheus scraper on
// the host network can read it.
type Metrics struct {
	mu       sync.Mutex
	counters map[string]float64
	gauges   map[string]float64
}

// NewMetrics builds an empty registry.
func NewMetrics() *Metrics {
	return &Metrics{
		counters: map[string]float64{},
		gauges:   map[string]float64{},
	}
}

// IncCounter adds one to a named counter.
func (m *Metrics) IncCounter(name string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.counters[name]++
	m.mu.Unlock()
}

// SetGauge records the current value of a named gauge.
func (m *Metrics) SetGauge(name string, value float64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.gauges[name] = value
	m.mu.Unlock()
}

// Middleware counts requests by method and status class.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		m.IncCounter(fmt.Sprintf("serverwatch_http_requests_total{method=%q,status_class=%q}", r.Method, statusClass(recorder.status)))
	})
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}

// Handler writes all counters and gauges in the Prometheus text format.
func (m *Metrics) Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	m.mu.Lock()
	defer m.mu.Unlock()

	var builder strings.Builder
	writeSamples(&builder, m.counters, "counter")
	writeSamples(&builder, m.gauges, "gauge")

	_, _ = w.Write([]byte(builder.String()))
}

func writeSamples(builder *strings.Builder, samples map[string]float64, kind string) {
	names := make([]string, 0, len(samples))
	for name := range samples {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		base := metricBaseName(name)
		builder.WriteString("# TYPE " + base + " " + kind + "\n")
		builder.WriteString(name + " " + strconv.FormatFloat(samples[name], 'f', -1, 64) + "\n")
	}
}

// metricBaseName strips a label set so the # TYPE line names the metric family.
func metricBaseName(name string) string {
	if index := strings.IndexByte(name, '{'); index >= 0 {
		return name[:index]
	}
	return name
}
