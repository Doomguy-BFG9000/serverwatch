package alert

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/config"
)

type Message struct {
	Subject string
	Body    string
	To      string
}

type Sender interface {
	Send(context.Context, Message) error
}

type SMTPSender struct{ Config config.AlertEmailConfig }

func (s SMTPSender) Send(ctx context.Context, message Message) error {
	if s.Config.Host == "" || s.Config.Port <= 0 || s.Config.From == "" || s.Config.To == "" {
		return fmt.Errorf("SMTP configuration is incomplete")
	}
	to := message.To
	if to == "" {
		to = s.Config.To
	}
	address := net.JoinHostPort(s.Config.Host, fmt.Sprint(s.Config.Port))
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(conn, s.Config.Host)
	if err != nil {
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer client.Close()
	if s.Config.TLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: s.Config.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	}
	if s.Config.Username != "" || s.Config.Password != "" {
		if err := client.Auth(smtp.PlainAuth("", s.Config.Username, s.Config.Password, s.Config.Host)); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}
	if err := client.Mail(s.Config.From); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP recipient failed: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP data failed: %w", err)
	}
	content := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", s.Config.From, to, message.Subject, strings.ReplaceAll(message.Body, "\n", "\r\n"))
	if _, err := writer.Write([]byte(content)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	return client.Quit()
}
