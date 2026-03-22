package email

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/smtp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSMTPClient implements SMTPClient for testing.
type mockSMTPClient struct {
	authFn     func(a smtp.Auth) error
	mailFn     func(from string) error
	rcptFn     func(to string) error
	dataFn     func() (io.WriteCloser, error)
	closeFn    func() error
	startTLSFn func(config *tls.Config) error
}

func (m *mockSMTPClient) Auth(a smtp.Auth) error {
	if m.authFn != nil {
		return m.authFn(a)
	}
	return nil
}

func (m *mockSMTPClient) Mail(from string) error {
	if m.mailFn != nil {
		return m.mailFn(from)
	}
	return nil
}

func (m *mockSMTPClient) Rcpt(to string) error {
	if m.rcptFn != nil {
		return m.rcptFn(to)
	}
	return nil
}

func (m *mockSMTPClient) Data() (io.WriteCloser, error) {
	if m.dataFn != nil {
		return m.dataFn()
	}
	return &mockWriteCloser{}, nil
}

func (m *mockSMTPClient) Close() error {
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func (m *mockSMTPClient) StartTLS(config *tls.Config) error {
	if m.startTLSFn != nil {
		return m.startTLSFn(config)
	}
	return nil
}

// mockWriteCloser captures written data for verification.
type mockWriteCloser struct {
	buf    strings.Builder
	closed bool
}

func (m *mockWriteCloser) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}

func (m *mockWriteCloser) Close() error {
	m.closed = true
	return nil
}

// mockDialer implements Dialer for testing.
type mockDialer struct {
	dialFn    func(ctx context.Context, addr string) (SMTPClient, error)
	dialTLSFn func(ctx context.Context, addr string, tlsConfig *tls.Config) (SMTPClient, error)
}

func (m *mockDialer) Dial(ctx context.Context, addr string) (SMTPClient, error) {
	if m.dialFn != nil {
		return m.dialFn(ctx, addr)
	}
	return &mockSMTPClient{}, nil
}

func (m *mockDialer) DialTLS(ctx context.Context, addr string, tlsConfig *tls.Config) (SMTPClient, error) {
	if m.dialTLSFn != nil {
		return m.dialTLSFn(ctx, addr, tlsConfig)
	}
	return &mockSMTPClient{}, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSender_Send_None_HappyPath(t *testing.T) {
	var (
		gotFrom string
		gotRcpt []string
		gotData string
	)

	writer := &mockWriteCloser{}
	client := &mockSMTPClient{
		mailFn: func(from string) error { gotFrom = from; return nil },
		rcptFn: func(to string) error { gotRcpt = append(gotRcpt, to); return nil },
		dataFn: func() (io.WriteCloser, error) { return writer, nil },
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, addr string) (SMTPClient, error) {
			assert.Equal(t, "mail.example.com:25", addr)
			return client, nil
		},
	}

	sender := NewSender("mail.example.com", 25, "alerts@example.com", "", "", "none", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com", "dev@example.com"}, "[Alert] Test", "<h1>Alert</h1>")
	require.NoError(t, err)

	assert.Equal(t, "alerts@example.com", gotFrom)
	assert.Equal(t, []string{"oncall@example.com", "dev@example.com"}, gotRcpt)

	gotData = writer.buf.String()
	assert.Contains(t, gotData, "From: alerts@example.com")
	assert.Contains(t, gotData, "To: oncall@example.com, dev@example.com")
	assert.Contains(t, gotData, "Subject: [Alert] Test")
	assert.Contains(t, gotData, "Date: ")
	assert.Contains(t, gotData, "Message-ID: <")
	assert.Contains(t, gotData, "MIME-Version: 1.0")
	assert.Contains(t, gotData, "Content-Type: text/html; charset=UTF-8")
	assert.Contains(t, gotData, "<h1>Alert</h1>")
	assert.True(t, writer.closed)
}

func TestSender_Send_STARTTLS_HappyPath(t *testing.T) {
	var startTLSCalled bool
	client := &mockSMTPClient{
		startTLSFn: func(config *tls.Config) error {
			startTLSCalled = true
			assert.Equal(t, "mail.example.com", config.ServerName)
			return nil
		},
		dataFn: func() (io.WriteCloser, error) { return &mockWriteCloser{}, nil },
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 587, "alerts@example.com", "", "", "starttls", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.NoError(t, err)
	assert.True(t, startTLSCalled, "StartTLS should have been called")
}

func TestSender_Send_TLS_HappyPath(t *testing.T) {
	var dialTLSCalled bool
	client := &mockSMTPClient{
		dataFn: func() (io.WriteCloser, error) { return &mockWriteCloser{}, nil },
	}

	dialer := &mockDialer{
		dialTLSFn: func(_ context.Context, addr string, config *tls.Config) (SMTPClient, error) {
			dialTLSCalled = true
			assert.Equal(t, "mail.example.com:465", addr)
			assert.Equal(t, "mail.example.com", config.ServerName)
			return client, nil
		},
	}

	sender := NewSender("mail.example.com", 465, "alerts@example.com", "", "", "tls", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.NoError(t, err)
	assert.True(t, dialTLSCalled, "DialTLS should have been called")
}

func TestSender_Send_WithAuth(t *testing.T) {
	var authCalled bool
	client := &mockSMTPClient{
		authFn: func(_ smtp.Auth) error {
			authCalled = true
			return nil
		},
		dataFn: func() (io.WriteCloser, error) { return &mockWriteCloser{}, nil },
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 587, "alerts@example.com", "user", "pass", "none", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.NoError(t, err)
	assert.True(t, authCalled, "Auth should have been called")
}

func TestSender_Send_WithoutAuth(t *testing.T) {
	var authCalled bool
	client := &mockSMTPClient{
		authFn: func(_ smtp.Auth) error {
			authCalled = true
			return nil
		},
		dataFn: func() (io.WriteCloser, error) { return &mockWriteCloser{}, nil },
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 25, "alerts@example.com", "", "", "none", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.NoError(t, err)
	assert.False(t, authCalled, "Auth should NOT have been called")
}

func TestSender_Send_DialError(t *testing.T) {
	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) {
			return nil, errors.New("connection refused")
		},
	}

	sender := NewSender("mail.example.com", 25, "alerts@example.com", "", "", "none", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email dial")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestSender_Send_DialTLSError(t *testing.T) {
	dialer := &mockDialer{
		dialTLSFn: func(_ context.Context, _ string, _ *tls.Config) (SMTPClient, error) {
			return nil, errors.New("tls handshake failed")
		},
	}

	sender := NewSender("mail.example.com", 465, "alerts@example.com", "", "", "tls", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email dial")
	assert.Contains(t, err.Error(), "tls handshake failed")
}

func TestSender_Send_StartTLSError(t *testing.T) {
	client := &mockSMTPClient{
		startTLSFn: func(_ *tls.Config) error {
			return errors.New("starttls not supported")
		},
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 587, "alerts@example.com", "", "", "starttls", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email STARTTLS")
}

func TestSender_Send_AuthError(t *testing.T) {
	client := &mockSMTPClient{
		authFn: func(_ smtp.Auth) error {
			return errors.New("invalid credentials")
		},
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 587, "alerts@example.com", "user", "pass", "none", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email auth")
}

func TestSender_Send_MailError(t *testing.T) {
	client := &mockSMTPClient{
		mailFn: func(_ string) error {
			return errors.New("sender rejected")
		},
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 25, "alerts@example.com", "", "", "none", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email MAIL FROM")
}

func TestSender_Send_RcptError(t *testing.T) {
	client := &mockSMTPClient{
		rcptFn: func(_ string) error {
			return errors.New("recipient rejected")
		},
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 25, "alerts@example.com", "", "", "none", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email RCPT TO")
}

func TestSender_Send_DataError(t *testing.T) {
	client := &mockSMTPClient{
		dataFn: func() (io.WriteCloser, error) {
			return nil, errors.New("data command failed")
		},
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 25, "alerts@example.com", "", "", "none", testLogger(), WithDialer(dialer))

	err := sender.Send(context.Background(), []string{"oncall@example.com"}, "Test", "<p>test</p>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email DATA")
}

func TestBuildMessage(t *testing.T) {
	msg := buildMessage("alerts@example.com", []string{"a@b.com", "c@d.com"}, "[Alert] Test", "<h1>Hello</h1>")

	assert.Contains(t, msg, "From: alerts@example.com\r\n")
	assert.Contains(t, msg, "To: a@b.com, c@d.com\r\n")
	assert.Contains(t, msg, "Date: ")
	assert.Contains(t, msg, "Message-ID: <")
	assert.Contains(t, msg, "@example.com>\r\n")
	assert.Contains(t, msg, "Subject: [Alert] Test\r\n")
	assert.Contains(t, msg, "MIME-Version: 1.0\r\n")
	assert.Contains(t, msg, "Content-Type: text/html; charset=UTF-8\r\n")
	assert.Contains(t, msg, "\r\n\r\n<h1>Hello</h1>")

	t.Logf("message:\n%s", msg)
}

func TestSanitizeHeader(t *testing.T) {
	assert.Equal(t, "safe subject", sanitizeHeader("safe subject"))
	assert.Equal(t, "injectedBcc: attacker@evil.com header", sanitizeHeader("injected\r\nBcc: attacker@evil.com\r\n header"))
	assert.Equal(t, "newline only", sanitizeHeader("newline\n only"))
	assert.Equal(t, "cr only", sanitizeHeader("cr\r only"))
}

func TestGenerateMessageID(t *testing.T) {
	id := generateMessageID("alerts@example.com")
	assert.True(t, strings.HasPrefix(id, "<"))
	assert.True(t, strings.HasSuffix(id, "@example.com>"))
	assert.Len(t, id, 1+32+1+len("example.com")+1) // < + 32 hex + @ + domain + >

	// Uniqueness.
	id2 := generateMessageID("alerts@example.com")
	assert.NotEqual(t, id, id2)
}
