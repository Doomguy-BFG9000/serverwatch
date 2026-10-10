package service

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Password hashing parameters. PBKDF2-HMAC-SHA256 with a random per-password
// salt is used so the Community Edition needs no third-party crypto dependency.
// Iterations follow the OWASP 2023 recommendation for PBKDF2-HMAC-SHA256.
const (
	passwordMinLength = 8
	passwordMaxLength = 200
	pbkdf2Iterations  = 210_000
	pbkdf2SaltLength  = 16
	pbkdf2KeyLength   = 32
	pbkdf2Prefix      = "pbkdf2_sha256"
	displayNameLimit  = 150
)

var (
	ErrEmailRequired     = errors.New("email is required")
	ErrEmailInvalid      = errors.New("email is not valid")
	ErrPasswordTooShort  = fmt.Errorf("password must be at least %d characters", passwordMinLength)
	ErrPasswordTooLong   = fmt.Errorf("password must be at most %d characters", passwordMaxLength)
	ErrInvalidCredential = errors.New("invalid email or password")
)

// User is the safe, public representation of an account. The password hash is
// deliberately never included.
type User struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}

type AuthStatus struct {
	// AuthEnabled reports whether the backend enforces authentication.
	AuthEnabled bool `json:"auth_enabled"`
	// SetupRequired is true when authentication is enabled but no account exists
	// yet, meaning the first visitor must create the administrator account.
	SetupRequired bool `json:"setup_required"`
	// Authenticated is true when the caller presents a valid session.
	Authenticated bool  `json:"authenticated"`
	User          *User `json:"user,omitempty"`
}

// NormalizeEmail trims surrounding whitespace and lowercases the address so
// accounts are case-insensitive and unique regardless of how they were typed.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail normalizes and validates an email address.
func ValidateEmail(email string) (string, error) {
	normalized := NormalizeEmail(email)
	if normalized == "" {
		return "", ErrEmailRequired
	}
	if utf8.RuneCountInString(normalized) > 255 {
		return "", ErrEmailInvalid
	}
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized {
		return "", ErrEmailInvalid
	}
	return normalized, nil
}

// ValidateDisplayName trims and bounds an optional display name.
func ValidateDisplayName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if utf8.RuneCountInString(trimmed) > displayNameLimit {
		return "", errors.New("display name must be at most 150 characters")
	}
	return trimmed, nil
}

// ValidatePassword enforces the password length policy. Length is measured in
// runes so multi-byte characters are counted the way a user expects.
func ValidatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < passwordMinLength {
		return ErrPasswordTooShort
	}
	if length > passwordMaxLength {
		return ErrPasswordTooLong
	}
	return nil
}

// HashPassword derives a PBKDF2-HMAC-SHA256 hash and encodes it as
// "pbkdf2_sha256$<iterations>$<salt-b64>$<key-b64>" so the parameters travel
// with the hash and can be tuned later without breaking existing accounts.
func HashPassword(password string) (string, error) {
	salt := make([]byte, pbkdf2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, pbkdf2KeyLength)
	if err != nil {
		return "", fmt.Errorf("derive password hash: %w", err)
	}
	return fmt.Sprintf(
		"%s$%d$%s$%s",
		pbkdf2Prefix,
		pbkdf2Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the encoded hash. It is
// constant-time with respect to the derived key and never panics on malformed
// input, so a corrupted row simply fails authentication.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != pbkdf2Prefix {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) == 0 {
		return false
	}
	derived, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(derived, expected) == 1
}

// NewSessionToken returns a URL-safe random token. The plaintext token is shown
// to the client once; only its hash is stored in the database.
func NewSessionToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// HashSessionToken returns the hex-encoded SHA-256 of a session token. Sessions
// are looked up by this value, so a database leak does not expose usable tokens.
func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
