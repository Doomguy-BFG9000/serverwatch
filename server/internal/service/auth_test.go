package service

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !strings.HasPrefix(hash, pbkdf2Prefix+"$") {
		t.Fatalf("hash does not carry its scheme prefix: %q", hash)
	}
	if !VerifyPassword(hash, "correct horse battery") {
		t.Fatal("correct password should verify")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("wrong password must not verify")
	}
}

func TestHashPasswordUsesRandomSalt(t *testing.T) {
	first, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	second, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if first == second {
		t.Fatal("hashes of equal passwords must differ (random salt)")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	cases := []string{
		"",
		"plaintext",
		"pbkdf2_sha256$notanumber$c2FsdA$a2V5",
		"pbkdf2_sha256$1000$!!!notbase64!!!$a2V5",
		"bcrypt$1000$c2FsdA$a2V5",
	}
	for _, encoded := range cases {
		if VerifyPassword(encoded, "anything") {
			t.Fatalf("malformed hash %q must not verify", encoded)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	valid := []struct {
		in   string
		want string
	}{
		{"  Admin@Example.COM ", "admin@example.com"},
		{"user@example.com", "user@example.com"},
		{"a@b", "a@b"},
	}
	for _, tc := range valid {
		got, err := ValidateEmail(tc.in)
		if err != nil {
			t.Fatalf("ValidateEmail(%q) unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ValidateEmail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	invalid := []string{"", "   ", "not-an-email", "@example.com", "user@", "user@example.com x"}
	for _, in := range invalid {
		if _, err := ValidateEmail(in); err == nil {
			t.Fatalf("ValidateEmail(%q) should fail", in)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Fatal("short password should fail")
	}
	if err := ValidatePassword(strings.Repeat("a", 201)); err == nil {
		t.Fatal("over-long password should fail")
	}
	if err := ValidatePassword("longenough"); err != nil {
		t.Fatalf("valid password rejected: %v", err)
	}
}

func TestSessionTokenHash(t *testing.T) {
	token, err := NewSessionToken()
	if err != nil {
		t.Fatalf("new session token: %v", err)
	}
	if len(token) < 32 {
		t.Fatalf("session token too short: %d", len(token))
	}
	if HashSessionToken(token) == token {
		t.Fatal("session token hash must not equal the plaintext token")
	}
	if HashSessionToken(token) != HashSessionToken(token) {
		t.Fatal("session token hash must be deterministic")
	}
}
