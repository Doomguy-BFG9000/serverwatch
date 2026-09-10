package httpapi

import (
	"net/http"

	"github.com/serverwatch/serverwatch/server/internal/config"
)

type AlertHandler struct{ Config config.AlertEmailConfig }

func (h *AlertHandler) Status(w http.ResponseWriter, r *http.Request) {
	c := h.Config
	writeJSON(w, http.StatusOK, map[string]any{
		"email_enabled":        c.Enabled,
		"smtp_configured":      c.Host != "" && c.Port > 0 && c.From != "",
		"recipient_configured": c.To != "",
	})
}
