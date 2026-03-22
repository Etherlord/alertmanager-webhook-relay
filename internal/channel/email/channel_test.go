package email

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannel_Name(t *testing.T) {
	ch := NewChannel(nil, nil, "", testLogger())
	assert.Equal(t, "email", ch.Name())
}

func TestChannel_Send_HappyPath(t *testing.T) {
	var sentTo []string
	var sentSubject string
	var sentBody string

	writer := &mockWriteCloser{}
	client := &mockSMTPClient{
		mailFn: func(_ string) error { return nil },
		rcptFn: func(to string) error { sentTo = append(sentTo, to); return nil },
		dataFn: func() (io.WriteCloser, error) { return writer, nil },
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 25, "alerts@example.com", "", "", "none", testLogger(), WithDialer(dialer))
	ch := NewChannel(sender, []string{"oncall@example.com", "dev@example.com"}, "[Alert]", testLogger())

	n := firingNotification()
	err := ch.Send(context.Background(), n)
	require.NoError(t, err)

	assert.Equal(t, []string{"oncall@example.com", "dev@example.com"}, sentTo)

	sentSubject = writer.buf.String()
	assert.Contains(t, sentSubject, "Subject: [Alert] [FIRING:1] HighCPU (webhook)")

	sentBody = writer.buf.String()
	assert.Contains(t, sentBody, "<!DOCTYPE html>")
	assert.Contains(t, sentBody, "HighCPU")
}

func TestChannel_Send_Error(t *testing.T) {
	client := &mockSMTPClient{
		mailFn: func(_ string) error {
			return errors.New("smtp failure")
		},
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 25, "alerts@example.com", "", "", "none", testLogger(), WithDialer(dialer))
	ch := NewChannel(sender, []string{"oncall@example.com"}, "[Alert]", testLogger())

	n := firingNotification()
	err := ch.Send(context.Background(), n)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email send")

	t.Logf("error: %v", err)
}

func TestChannel_Send_ResolvedNotification(t *testing.T) {
	writer := &mockWriteCloser{}
	client := &mockSMTPClient{
		dataFn: func() (io.WriteCloser, error) { return writer, nil },
	}

	dialer := &mockDialer{
		dialFn: func(_ context.Context, _ string) (SMTPClient, error) { return client, nil },
	}

	sender := NewSender("mail.example.com", 25, "alerts@example.com", "", "", "none", testLogger(), WithDialer(dialer))
	ch := NewChannel(sender, []string{"oncall@example.com"}, "[Alert]", testLogger())

	n := resolvedNotification()
	err := ch.Send(context.Background(), n)
	require.NoError(t, err)

	msg := writer.buf.String()
	assert.Contains(t, msg, "Subject: [Alert] [RESOLVED:1] HighCPU (webhook)")
	assert.Contains(t, msg, "RESOLVED")
}
