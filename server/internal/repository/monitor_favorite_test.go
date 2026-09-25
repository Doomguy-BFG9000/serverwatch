package repository

import (
	"strings"
	"testing"
)

// TestSetFavoriteQueryIsIsolated verifies the favorite toggle only writes
// monitors.favorite. Marking a monitor as a favorite is metadata-only, so the
// query must not touch check history, alert state, or any other monitor column.
func TestSetFavoriteQueryIsIsolated(t *testing.T) {
	query := strings.ToLower(setFavoriteQuery)

	if !strings.Contains(query, "update monitors") {
		t.Fatal("SetFavorite must update the monitors table")
	}
	if !strings.Contains(query, "favorite = $2") {
		t.Fatal("SetFavorite must write the favorite column")
	}

	for _, forbidden := range []string{
		"monitor_alert_state",
		"monitor_results",
		"insert",
		"delete",
		"enabled =",
		"maintenance =",
		"group_id =",
		"config =",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("SetFavorite query must not reference %q", forbidden)
		}
	}
}

// TestListDueQueryIgnoresFavorite guards the Phase 0.46 requirement that
// favorites have zero effect on scheduler eligibility.
func TestListDueQueryIgnoresFavorite(t *testing.T) {
	query := strings.ToLower(listDueQuery)

	if strings.Contains(query, "favorite") {
		t.Fatal("ListDue query must not filter on favorite")
	}
}
