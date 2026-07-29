package mail

import (
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/config"
)

// startFakeSMTP runs a minimal single-connection SMTP server on a loopback
// port and returns the host, port and a channel delivering the raw DATA
// payload (headers + body) of the first message received.
func startFakeSMTP(t *testing.T) (string, int, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	dataCh := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		tp := textproto.NewConn(conn)
		_ = tp.PrintfLine("220 fake ESMTP ready")
		for {
			line, err := tp.ReadLine()
			if err != nil {
				return
			}
			cmd := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				// Advertise AUTH PLAIN: SendMail requires it when auth is
				// non-nil, and PlainAuth allows plaintext on 127.0.0.1.
				_ = tp.PrintfLine("250-fake")
				_ = tp.PrintfLine("250 AUTH PLAIN")
			case strings.HasPrefix(cmd, "AUTH"):
				_ = tp.PrintfLine("235 authenticated")
			case strings.HasPrefix(cmd, "DATA"):
				_ = tp.PrintfLine("354 send data")
				payload, err := tp.ReadDotBytes()
				if err != nil {
					return
				}
				select {
				case dataCh <- string(payload):
				default:
				}
				_ = tp.PrintfLine("250 accepted")
			case strings.HasPrefix(cmd, "QUIT"):
				_ = tp.PrintfLine("221 bye")
				return
			default: // MAIL FROM / RCPT TO / RSET / NOOP
				_ = tp.PrintfLine("250 OK")
			}
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port, dataCh
}

// receive waits for the fake server to hand over the captured message.
func receive(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case data := <-ch:
		return data
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SMTP payload")
		return ""
	}
}

func TestSend_NoRecipients(t *testing.T) {
	m := New(config.MailConfig{Host: "smtp.example.com", Port: 587})
	err := m.Send(&Message{To: nil, Subject: "x", Body: "y"})
	if err == nil || !strings.Contains(err.Error(), "no recipients") {
		t.Fatalf("expected 'no recipients' error, got %v", err)
	}
}

func TestSend_NoHostConfigured_SkipsSilently(t *testing.T) {
	m := New(config.MailConfig{})
	if err := m.Send(&Message{To: []string{"a@b.c"}, Subject: "x", Body: "y"}); err != nil {
		t.Fatalf("unconfigured SMTP should skip without error, got %v", err)
	}
}

func TestSend_HeadersAndBody(t *testing.T) {
	host, port, dataCh := startFakeSMTP(t)
	m := New(config.MailConfig{
		Host: host, Port: port,
		From: "noreply@contentx.io", FromName: "ContentX",
	})

	err := m.Send(&Message{
		To:      []string{"one@example.com", "two@example.com"},
		Subject: "Greetings",
		Body:    "hello body",
		HTML:    false,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	data := receive(t, dataCh)
	for _, want := range []string{
		"From: ContentX <noreply@contentx.io>",
		"To: one@example.com, two@example.com",
		"Subject: Greetings",
		"Content-Type: text/plain; charset=UTF-8",
		"MIME-Version: 1.0",
		"hello body",
	} {
		if !strings.Contains(data, want) {
			t.Errorf("payload missing %q\npayload:\n%s", want, data)
		}
	}
}

func TestSend_HTMLContentType(t *testing.T) {
	host, port, dataCh := startFakeSMTP(t)
	// From empty — falls back to User.
	m := New(config.MailConfig{Host: host, Port: port, User: "user@contentx.io"})

	err := m.Send(&Message{
		To:      []string{"x@example.com"},
		Subject: "HTML mail",
		Body:    "<b>bold</b>",
		HTML:    true,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	data := receive(t, dataCh)
	if !strings.Contains(data, "Content-Type: text/html; charset=UTF-8") {
		t.Errorf("expected html content type, payload:\n%s", data)
	}
	if !strings.Contains(data, "From:  <user@contentx.io>") {
		t.Errorf("From should fall back to cfg.User, payload:\n%s", data)
	}
}

func TestSendVerification_RendersTemplate(t *testing.T) {
	host, port, dataCh := startFakeSMTP(t)
	m := New(config.MailConfig{Host: host, Port: port, From: "noreply@contentx.io"})

	if err := m.SendVerification("new@example.com", "alice", "https://cx.io/verify?t=abc"); err != nil {
		t.Fatalf("SendVerification: %v", err)
	}

	data := receive(t, dataCh)
	for _, want := range []string{
		"Subject: Verify your email address",
		"Hi alice,",
		"https://cx.io/verify?t=abc",
		"expires in 24 hours",
	} {
		if !strings.Contains(data, want) {
			t.Errorf("verification mail missing %q\npayload:\n%s", want, data)
		}
	}
}

func TestSendPasswordReset_RendersTemplate(t *testing.T) {
	host, port, dataCh := startFakeSMTP(t)
	m := New(config.MailConfig{Host: host, Port: port, From: "noreply@contentx.io"})

	if err := m.SendPasswordReset("u@example.com", "bob", "https://cx.io/reset?t=xyz"); err != nil {
		t.Fatalf("SendPasswordReset: %v", err)
	}

	data := receive(t, dataCh)
	for _, want := range []string{
		"Subject: Reset your password",
		"Hi bob,",
		"https://cx.io/reset?t=xyz",
		"expires in 1 hour",
		"If you didn't request this, ignore this email.",
	} {
		if !strings.Contains(data, want) {
			t.Errorf("reset mail missing %q\npayload:\n%s", want, data)
		}
	}
}

func TestSendCommentNotification_RendersTemplate(t *testing.T) {
	host, port, dataCh := startFakeSMTP(t)
	m := New(config.MailConfig{Host: host, Port: port, From: "noreply@contentx.io"})

	if err := m.SendCommentNotification("author@example.com", "carol", "My Post", "Nice article!"); err != nil {
		t.Fatalf("SendCommentNotification: %v", err)
	}

	data := receive(t, dataCh)
	for _, want := range []string{
		"Subject: New comment on: My Post",
		"Hi carol,",
		`A new comment was posted on "My Post"`,
		"Nice article!",
		"Log in to moderate.",
	} {
		if !strings.Contains(data, want) {
			t.Errorf("comment mail missing %q\npayload:\n%s", want, data)
		}
	}
}
