package httpapi

import (
	"net/http"
	"strings"

	"github.com/serverwatch/serverwatch/server/internal/config"
)

type AlertHandler struct {
	Config         config.AlertEmailConfig
	TelegramConfig config.AlertTelegramConfig
}

func (h *AlertHandler) Status(w http.ResponseWriter, r *http.Request) {
	c := h.Config
	t := h.TelegramConfig
	writeJSON(w, http.StatusOK, map[string]any{
		"email_enabled":            c.Enabled,
		"smtp_configured":          c.Host != "" && c.Port > 0 && c.From != "",
		"recipient_configured":     c.To != "",
		"telegram_enabled":         t.Enabled,
		"telegram_configured":      t.BotToken != "" && t.ChatID != "",
		"telegram_chat_configured": t.ChatID != "",
		// Non-secret delivery details for the read-only Alerts panel. SMTP
		// password and the Telegram bot token are never included.
		"smtp_host":         c.Host,
		"smtp_port":         c.Port,
		"smtp_tls":          c.TLS,
		"email_from":        c.From,
		"email_recipients":  splitRecipients(c.To),
		"telegram_chat_id":  t.ChatID,
		"telegram_api_base": t.APIBase,
	})
}

// splitRecipients turns a comma-separated SMTP_TO value into a trimmed list,
// dropping empty entries so the dashboard can render each recipient clearly.
func splitRecipients(raw string) []string {
	recipients := []string{}
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			recipients = append(recipients, trimmed)
		}
	}
	return recipients
}
