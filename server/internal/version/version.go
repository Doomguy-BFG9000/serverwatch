// Package version exposes build metadata for the ServerWatch backend.
//
// The values are placeholders in source control and are overridden at build
// time with -ldflags -X (see the Makefile and scripts/release.sh). Exposing
// them lets operators confirm exactly which build is running via /health,
// /ready, and the dashboard, without guessing from file timestamps.
package version

import "fmt"

var (
	// Version is the release tag (for example "v0.1.0") or "dev" for a local build.
	Version = "dev"
	// Commit is the short git revision the binary was built from.
	Commit = "unknown"
	// Date is the UTC build timestamp in RFC 3339 form.
	Date = "unknown"
)

// String returns a compact build identifier such as "v0.1.0 (a1b2c3d)".
// It falls back to just the version when no commit is recorded.
func String() string {
	if Commit == "" || Commit == "unknown" {
		return Version
	}
	return fmt.Sprintf("%s (%s)", Version, Commit)
}
