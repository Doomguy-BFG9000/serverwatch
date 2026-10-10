package version

import "testing"

func TestStringIncludesCommit(t *testing.T) {
	original := Version
	originalCommit := Commit
	defer func() { Version = original; Commit = originalCommit }()

	Version = "v1.2.3"
	Commit = "abc1234"
	if got := String(); got != "v1.2.3 (abc1234)" {
		t.Fatalf("String() = %q", got)
	}
}

func TestStringWithoutCommit(t *testing.T) {
	original := Version
	originalCommit := Commit
	defer func() { Version = original; Commit = originalCommit }()

	Version = "dev"
	Commit = "unknown"
	if got := String(); got != "dev" {
		t.Fatalf("String() = %q", got)
	}
}
