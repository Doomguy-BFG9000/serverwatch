package repository

import (
	"strings"
	"testing"
)

// TestPruneResultsQueryKeepsLatestPerMonitor guards the core retention
// invariant: pruning deletes only rows older than the cutoff and never removes
// the newest result for a monitor. Losing the latest row would blank the
// dashboard's last status and make the due-scheduler re-check immediately.
func TestPruneResultsQueryKeepsLatestPerMonitor(t *testing.T) {
	query := strings.ToLower(pruneResultsQuery)

	required := []string{
		"delete from monitor_results",
		"checked_at < now() - make_interval(days => $1)",
		"select distinct on (monitor_id) id",
		"order by monitor_id, checked_at desc",
		"not in",
	}
	for _, fragment := range required {
		if !strings.Contains(query, fragment) {
			t.Fatalf("prune query must contain %q", fragment)
		}
	}
}

// TestPruneResultsQueryIsBounded confirms the delete is scoped to
// monitor_results and cannot touch monitors, sessions, or alert state.
func TestPruneResultsQueryIsBounded(t *testing.T) {
	query := strings.ToLower(pruneResultsQuery)

	for _, forbidden := range []string{"monitors\n", "update ", "monitor_alert_state", "sessions"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("prune query must not reference %q", forbidden)
		}
	}
}
