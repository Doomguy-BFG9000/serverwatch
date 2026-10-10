package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/config"
)

// telegramMessageLimit is Telegram's maximum message length. Longer alerts are
// truncated rather than rejected, because a missed DOWN notification is worse
// than a clipped one.
const telegramMessageLimit = 4096

// TelegramSender delivers alerts through the Telegram Bot API. It implements
// the same Sender interface as SMTPSender, so a deployment can enable either or
// both.
type TelegramSender struct {
	Config config.AlertTelegramConfig
	Client *http.Client
}

// Send posts the alert to the configured chat. The bot token and chat id are
// validated up front so a misconfigured sender returns a clear error instead of
// an opaque HTTP status.
func (s TelegramSender) Send(ctx context.Context, message Message) error {
	if s.Config.BotToken == "" || s.Config.ChatID == "" {
		return fmt.Errorf("telegram configuration is incomplete")
	}
	apiBase := s.Config.APIBase
	if apiBase == "" {
		apiBase = "https://api.telegram.org"
	}
	apiBase = strings.TrimRight(apiBase, "/")

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	text := message.Subject
	if message.Body != "" {
		text = message.Subject + "\n\n" + message.Body
	}
	if len(text) > telegramMessageLimit {
		text = text[:telegramMessageLimit]
	}

	payload, err := json.Marshal(map[string]any{
		"chat_id":                  s.Config.ChatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return fmt.Errorf("encode telegram payload: %w", err)
	}

	// The token is part of the path; never include the URL in returned errors so
	// the secret cannot leak into logs.
	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", apiBase, s.Config.BotToken)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		// A *url.Error stringifies the full URL, which contains the bot token.
		// Unwrap it so the secret cannot reach the logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return fmt.Errorf("send telegram message: %w", urlErr.Err)
		}
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(response.Body, 8<<10))

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("telegram returned an unreadable response (status %d)", response.StatusCode)
	}
	if !result.OK {
		description := result.Description
		if description == "" {
			description = fmt.Sprintf("status %d", response.StatusCode)
		}
		return fmt.Errorf("telegram rejected the message: %s", description)
	}
	return nil
}
