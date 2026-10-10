package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr       string
	APIToken       string
	AuthEnabled    bool
	SessionTTL     time.Duration
	CookieSecure   bool
	MetricsEnabled bool
	RateLimitRPS   float64
	RateLimitBurst int
	TrustProxy     bool
	RetentionDays  int
	AlertEmail     AlertEmailConfig
}

type AlertEmailConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       string
	TLS      bool
}

func Load() Config {
	addr := os.Getenv("SERVERWATCH_HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	port := envInt("SMTP_PORT", 587)
	tls := envBool("SMTP_TLS", true)
	alerts := AlertEmailConfig{
		Enabled: envBool("SERVERWATCH_ALERT_EMAIL_ENABLED", false),
		Host:    os.Getenv("SMTP_HOST"), Port: port,
		Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"),
		From: os.Getenv("SMTP_FROM"), To: os.Getenv("SMTP_TO"), TLS: tls,
	}
	if alerts.Enabled && (alerts.Host == "" || alerts.Port <= 0 || alerts.From == "" || alerts.To == "") {
		log.Printf("alert email is enabled but SMTP_HOST, SMTP_PORT, SMTP_FROM, and SMTP_TO must be configured")
	}
	return Config{
		HTTPAddr:       addr,
		APIToken:       strings.TrimSpace(os.Getenv("SERVERWATCH_API_TOKEN")),
		AuthEnabled:    envBool("SERVERWATCH_AUTH_ENABLED", false),
		SessionTTL:     envDuration("SERVERWATCH_SESSION_TTL", 30*24*time.Hour),
		CookieSecure:   envBool("SERVERWATCH_COOKIE_SECURE", false),
		MetricsEnabled: envBool("SERVERWATCH_METRICS_ENABLED", false),
		RateLimitRPS:   envFloat("SERVERWATCH_RATE_LIMIT_RPS", 10),
		RateLimitBurst: envInt("SERVERWATCH_RATE_LIMIT_BURST", 30),
		TrustProxy:     envBool("SERVERWATCH_TRUST_PROXY", true),
		RetentionDays:  envInt("SERVERWATCH_RESULT_RETENTION_DAYS", 30),
		AlertEmail:     alerts,
	}
}

// envDuration parses a Go duration string (for example "720h" or "30m"). An
// empty or invalid value returns the fallback.
func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

// envFloat parses a positive floating-point value used for rates. An empty,
// invalid, or non-positive value returns the fallback.
func envFloat(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
