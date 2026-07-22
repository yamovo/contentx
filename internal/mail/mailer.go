package mail

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/yamovo/contentx/internal/config"
)

// Mailer handles sending emails via SMTP.
type Mailer struct {
	cfg config.MailConfig
}

// New creates a new Mailer instance.
func New(cfg config.MailConfig) *Mailer {
	return &Mailer{cfg: cfg}
}

// Message represents an email message.
type Message struct {
	To      []string
	Subject string
	Body    string
	HTML    bool
}

// Send sends an email message.
func (m *Mailer) Send(msg *Message) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients")
	}

	if m.cfg.Host == "" {
		slog.Warn("SMTP not configured, skipping email",
			"to", msg.To, "subject", msg.Subject)
		return nil
	}

	from := m.cfg.From
	if from == "" {
		from = m.cfg.User
	}

	headers := map[string]string{
		"From":         fmt.Sprintf("%s <%s>", m.cfg.FromName, from),
		"To":           strings.Join(msg.To, ", "),
		"Subject":      msg.Subject,
		"MIME-Version": "1.0",
		"Date":         time.Now().Format(time.RFC1123Z),
	}

	if msg.HTML {
		headers["Content-Type"] = "text/html; charset=UTF-8"
	} else {
		headers["Content-Type"] = "text/plain; charset=UTF-8"
	}

	var sb strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&sb, "%s: %s\r\n", k, v)
	}
	sb.WriteString("\r\n")
	sb.WriteString(msg.Body)

	body := sb.String()

	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	auth := smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)

	if m.cfg.UseTLS {
		return m.sendTLS(addr, auth, from, msg.To, body)
	}

	return smtp.SendMail(addr, auth, from, msg.To, []byte(body))
}

// sendTLS sends email with explicit TLS (port 465).
func (m *Mailer) sendTLS(addr string, auth smtp.Auth, from string, to []string, body string) error {
	host, _, _ := net.SplitHostPort(addr)

	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(body)); err != nil {
		return err
	}

	return w.Close()
}

// SendVerification sends an email verification link.
func (m *Mailer) SendVerification(to, username, verifyURL string) error {
	return m.Send(&Message{
		To:      []string{to},
		Subject: "Verify your email address",
		Body:    fmt.Sprintf("Hi %s,\n\nPlease verify your email by clicking:\n%s\n\nThis link expires in 24 hours.", username, verifyURL),
		HTML:    false,
	})
}

// SendPasswordReset sends a password reset link.
func (m *Mailer) SendPasswordReset(to, username, resetURL string) error {
	return m.Send(&Message{
		To:      []string{to},
		Subject: "Reset your password",
		Body:    fmt.Sprintf("Hi %s,\n\nReset your password:\n%s\n\nThis link expires in 1 hour.\nIf you didn't request this, ignore this email.", username, resetURL),
		HTML:    false,
	})
}

// SendCommentNotification notifies an author about a new comment.
func (m *Mailer) SendCommentNotification(to, authorName, articleTitle, commentPreview string) error {
	return m.Send(&Message{
		To:      []string{to},
		Subject: fmt.Sprintf("New comment on: %s", articleTitle),
		Body:    fmt.Sprintf("Hi %s,\n\nA new comment was posted on \"%s\":\n\n%s\n\nLog in to moderate.", authorName, articleTitle, commentPreview),
		HTML:    false,
	})
}
