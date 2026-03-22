package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"
)

const (
	defaultDialTimeout = 10 * time.Second
)

// SMTPClient abstracts *smtp.Client operations for testing.
type SMTPClient interface {
	Auth(a smtp.Auth) error
	Mail(from string) error
	Rcpt(to string) error
	Data() (io.WriteCloser, error)
	Close() error
	StartTLS(config *tls.Config) error
}

// Dialer creates SMTP connections.
type Dialer interface {
	Dial(ctx context.Context, addr string) (SMTPClient, error)
	DialTLS(ctx context.Context, addr string, tlsConfig *tls.Config) (SMTPClient, error)
}

// SenderOption configures the Sender.
type SenderOption func(*Sender)

// WithDialer sets a custom Dialer (useful for testing).
func WithDialer(d Dialer) SenderOption {
	return func(s *Sender) {
		s.dialer = d
	}
}

// Sender sends emails via SMTP.
type Sender struct {
	host     string
	port     int
	from     string
	username string
	password string
	tlsMode  string
	logger   *slog.Logger
	dialer   Dialer
}

// NewSender creates a new SMTP Sender.
func NewSender(host string, port int, from, username, password, tlsMode string, logger *slog.Logger, opts ...SenderOption) *Sender {
	s := &Sender{
		host:     host,
		port:     port,
		from:     from,
		username: username,
		password: password,
		tlsMode:  tlsMode,
		logger:   logger,
		dialer:   &netDialer{timeout: defaultDialTimeout},
	}
	for _, opt := range opts {
		opt(s)
	}
	logger.Debug("email sender created",
		"host", host,
		"port", port,
		"from", from,
		"tls_mode", tlsMode,
	)
	return s
}

// Send sends an email with the given subject and HTML body to the specified recipients.
func (s *Sender) Send(ctx context.Context, to []string, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.logger.Debug("email sender: dialing", "addr", addr, "tls_mode", s.tlsMode)

	client, err := s.dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("email dial %s: %w", addr, err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			s.logger.Debug("email sender: close error", "error", err)
		}
	}()

	// STARTTLS upgrade if needed.
	if s.tlsMode == "starttls" {
		s.logger.Debug("email sender: upgrading to STARTTLS", "host", s.host)
		//nolint:gosec // TLS config is intentionally using the SMTP host for ServerName
		tlsConfig := &tls.Config{ServerName: s.host}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("email STARTTLS: %w", err)
		}
	}

	// Authenticate if credentials are provided.
	if s.username != "" && s.password != "" {
		s.logger.Debug("email sender: authenticating", "username", s.username)
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email auth: %w", err)
		}
	}

	// Set sender.
	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("email MAIL FROM: %w", err)
	}

	// Set recipients.
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("email RCPT TO %s: %w", rcpt, err)
		}
	}

	// Write message.
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("email DATA: %w", err)
	}

	msg := buildMessage(s.from, to, subject, htmlBody)
	if _, err := io.WriteString(w, msg); err != nil {
		return fmt.Errorf("email write body: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("email close data: %w", err)
	}

	s.logger.Debug("email sender: message sent",
		"to", to,
		"subject", subject,
	)
	return nil
}

// dial creates an SMTP client using the appropriate TLS mode.
func (s *Sender) dial(ctx context.Context, addr string) (SMTPClient, error) {
	switch s.tlsMode {
	case "tls":
		//nolint:gosec // TLS config is intentionally using the SMTP host for ServerName
		tlsConfig := &tls.Config{ServerName: s.host}
		return s.dialer.DialTLS(ctx, addr, tlsConfig)
	default: // "starttls", "none"
		return s.dialer.Dial(ctx, addr)
	}
}

// buildMessage constructs an RFC 2822 email message with HTML content.
func buildMessage(from string, to []string, subject, htmlBody string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: %s\r\n", generateMessageID(from))
	fmt.Fprintf(&b, "Subject: %s\r\n", sanitizeHeader(subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return b.String()
}

// sanitizeHeader strips CR and LF characters to prevent SMTP header injection.
func sanitizeHeader(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// generateMessageID creates a unique RFC 5322 Message-ID using crypto/rand.
func generateMessageID(from string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	domain := from
	if idx := strings.LastIndex(from, "@"); idx >= 0 {
		domain = from[idx+1:]
	}
	domain = strings.TrimRight(domain, ">")
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(b), domain)
}

// netDialer is the default Dialer using net.Dialer and tls.Dialer.
type netDialer struct {
	timeout time.Duration
}

func (d *netDialer) Dial(ctx context.Context, addr string) (SMTPClient, error) {
	dialer := &net.Dialer{Timeout: d.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	host, _, _ := net.SplitHostPort(addr)
	return smtp.NewClient(conn, host)
}

func (d *netDialer) DialTLS(ctx context.Context, addr string, tlsConfig *tls.Config) (SMTPClient, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: d.timeout},
		Config:    tlsConfig,
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	host, _, _ := net.SplitHostPort(addr)
	return smtp.NewClient(conn, host)
}
