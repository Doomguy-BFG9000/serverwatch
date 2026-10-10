package httpapi

// Public status page (Phase 0.55, opt-in).
//
// When SERVERWATCH_STATUS_PAGE_ENABLED=true the backend serves a read-only,
// server-rendered HTML summary at /status (with a stylesheet at /status.css).
// It exposes only monitor names, their derived state, response time, and last
// check time — never targets, config, messages, or credentials — so it is safe
// to share with users or embed behind a public reverse-proxy location.
//
// The page is deliberately self-contained: it reuses the existing
// MonitorStore.ListStatus query, adds no database schema, no external assets,
// and no JavaScript, so it respects the strict Content-Security-Policy set by
// SecurityHeaders (style-src 'self', script-src 'self').
//
// The page is off by default; when disabled the routes return 404 so an install
// never changes behavior until an operator opts in.

import (
	"context"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

// StatusStore is the narrow read surface the status page needs. Change-detector
// tests keep it aligned with MonitorStore.
type StatusStore interface {
	ListStatus(context.Context) ([]service.MonitorStatus, error)
}

// StatusPageHandler renders the opt-in public status page.
type StatusPageHandler struct {
	Repository StatusStore
	Enabled    bool
}

// statusPageMonitor is the sanitized projection rendered for one monitor.
type statusPageMonitor struct {
	Name         string
	Type         string
	State        string // up | down | pending | maintenance | disabled
	StateLabel   string
	ResponseTime string
	LastChecked  string
}

type statusPageGroup struct {
	Name     string
	Monitors []statusPageMonitor
}

type statusPageSummary struct {
	Up          int
	Down        int
	Pending     int
	Maintenance int
	Disabled    int
	Total       int
}

type statusPageData struct {
	Version     string
	GeneratedAt string
	Overall     string // operational | degraded | empty
	OverallText string
	Summary     statusPageSummary
	Groups      []statusPageGroup
}

var statusPageTemplate = template.Must(template.New("status").Parse(statusPageHTML))

var statusPageStylesheet = []byte(statusPageCSS)

// Status renders the HTML status page. It returns 404 when the feature is off.
func (h *StatusPageHandler) Status(w http.ResponseWriter, r *http.Request) {
	if !h.Enabled {
		http.NotFound(w, r)
		return
	}

	statuses, err := h.Repository.ListStatus(r.Context())
	if err != nil {
		http.Error(w, "failed to load monitor status", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	data := buildStatusPage(statuses, now)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_ = statusPageTemplate.Execute(w, data)
}

// Stylesheet serves the status page CSS as a separate resource so the page can
// be styled under the strict `style-src 'self'` policy without inline styles.
func (h *StatusPageHandler) Stylesheet(w http.ResponseWriter, r *http.Request) {
	if !h.Enabled {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(statusPageStylesheet)
}

// buildStatusPage folds raw statuses into the rendered view. It mirrors the
// dashboard's precedence — disabled overrides maintenance, which overrides the
// stored check result — so the public page never disagrees with the dashboard.
func buildStatusPage(statuses []service.MonitorStatus, now time.Time) statusPageData {
	summary := statusPageSummary{Total: len(statuses)}

	grouped := map[string][]statusPageMonitor{}

	for _, status := range statuses {
		state := statusPageState(status)
		switch state {
		case "up":
			summary.Up++
		case "down":
			summary.Down++
		case "maintenance":
			summary.Maintenance++
		case "disabled":
			summary.Disabled++
		default: // pending / unknown
			summary.Pending++
		}

		groupName := strings.TrimSpace(status.GroupName)
		if groupName == "" {
			groupName = "Monitors"
		}
		grouped[groupName] = append(grouped[groupName], statusPageMonitor{
			Name:         status.Name,
			Type:         strings.ToUpper(status.MonitorType),
			State:        state,
			StateLabel:   statusStateLabel(state),
			ResponseTime: formatResponseTime(status.ResponseTimeMS),
			LastChecked:  humanizeSince(status.LastCheckedAt, now),
		})
	}

	groups := make([]statusPageGroup, 0, len(grouped))
	for name, monitors := range grouped {
		sort.Slice(monitors, func(i, j int) bool {
			return monitors[i].Name < monitors[j].Name
		})
		groups = append(groups, statusPageGroup{Name: name, Monitors: monitors})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

	overall := "operational"
	overallText := "All systems operational"
	if summary.Total == 0 {
		overall = "empty"
		overallText = "No monitors configured"
	} else if summary.Down > 0 {
		overall = "degraded"
		overallText = "Degraded service"
	}
	if summary.Down == 0 && summary.Pending > 0 && overall != "empty" {
		overallText = "All systems operational"
	}

	return statusPageData{
		Version:     statusPageVersion(),
		GeneratedAt: now.Format("2006-01-02 15:04 UTC"),
		Overall:     overall,
		OverallText: overallText,
		Summary:     summary,
		Groups:      groups,
	}
}

func statusPageState(status service.MonitorStatus) string {
	if !status.Enabled {
		return "disabled"
	}
	if status.Maintenance {
		return "maintenance"
	}
	switch status.Status {
	case "up", "down":
		return status.Status
	default:
		return "pending"
	}
}

func statusStateLabel(state string) string {
	switch state {
	case "up":
		return "Operational"
	case "down":
		return "Outage"
	case "maintenance":
		return "Maintenance"
	case "disabled":
		return "Disabled"
	default:
		return "No data"
	}
}

func formatResponseTime(ms *int64) string {
	if ms == nil {
		return "—"
	}
	return strconv.FormatInt(*ms, 10) + " ms"
}

// humanizeSince renders a coarse relative age suitable for a public page.
func humanizeSince(t *time.Time, now time.Time) string {
	if t == nil {
		return "never"
	}
	delta := now.Sub(*t)
	if delta < 0 {
		delta = 0
	}
	switch {
	case delta < time.Minute:
		return "just now"
	case delta < time.Hour:
		return strconv.Itoa(int(delta.Minutes())) + "m ago"
	case delta < 24*time.Hour:
		return strconv.Itoa(int(delta.Hours())) + "h ago"
	default:
		return strconv.Itoa(int(delta.Hours()/24)) + "d ago"
	}
}
