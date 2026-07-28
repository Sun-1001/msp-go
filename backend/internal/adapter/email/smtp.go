package emailadapter

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	emailapp "mathstudy/backend/internal/application/email"
)

const (
	defaultDialTimeout = 10 * time.Second
	defaultIOTimeout   = 20 * time.Second
)

// SMTPTransport sends mail through the standard SMTP protocol.
type SMTPTransport struct {
	dialTimeout time.Duration
	ioTimeout   time.Duration
}

// NewSMTPTransport creates an SMTP transport with bounded network timeouts.
func NewSMTPTransport() *SMTPTransport {
	return &SMTPTransport{dialTimeout: defaultDialTimeout, ioTimeout: defaultIOTimeout}
}

// Test connects, negotiates encryption, authenticates, and issues NOOP.
func (t *SMTPTransport) Test(ctx context.Context, config emailapp.SMTPConfig) error {
	client, closeClient, err := t.connect(ctx, config)
	if err != nil {
		return err
	}
	defer closeClient()
	if err := authenticate(client, config); err != nil {
		return err
	}
	if err := client.Noop(); err != nil {
		return fmt.Errorf("smtp noop: %w", err)
	}
	_ = client.Quit()
	return nil
}

// Send delivers one UTF-8 HTML message.
func (t *SMTPTransport) Send(ctx context.Context, config emailapp.SMTPConfig, message emailapp.Message) error {
	from, err := envelopeAddress(config.From, "from")
	if err != nil {
		return err
	}
	to, err := envelopeAddress(message.To, "recipient")
	if err != nil {
		return err
	}
	data, err := encodeMessage(config, to, message.Subject, message.HTMLBody)
	if err != nil {
		return err
	}
	client, closeClient, err := t.connect(ctx, config)
	if err != nil {
		return err
	}
	defer closeClient()
	if err := authenticate(client, config); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write smtp message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish smtp message: %w", err)
	}
	_ = client.Quit()
	return nil
}

func (t *SMTPTransport) connect(ctx context.Context, config emailapp.SMTPConfig) (*smtp.Client, func(), error) {
	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	dialer := &net.Dialer{Timeout: t.dialTimeout}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, nil, fmt.Errorf("smtp dial: %w", err)
	}
	closeConnection := func() { _ = connection.Close() }
	if err := connection.SetDeadline(connectionDeadline(ctx, t.ioTimeout)); err != nil {
		closeConnection()
		return nil, nil, fmt.Errorf("set smtp deadline: %w", err)
	}
	tlsConfig := &tls.Config{
		ServerName: config.Host,
		MinVersion: tls.VersionTLS12,
	}
	if config.UseTLS {
		tlsConnection := tls.Client(connection, tlsConfig)
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			closeConnection()
			return nil, nil, fmt.Errorf("smtp tls handshake: %w", err)
		}
		connection = tlsConnection
	}
	client, err := smtp.NewClient(connection, config.Host)
	if err != nil {
		closeConnection()
		return nil, nil, fmt.Errorf("create smtp client: %w", err)
	}
	closeClient := func() { _ = client.Close() }
	if !config.UseTLS {
		if supported, _ := client.Extension("STARTTLS"); !supported {
			closeClient()
			return nil, nil, fmt.Errorf("smtp server does not support STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			closeClient()
			return nil, nil, fmt.Errorf("smtp starttls: %w", err)
		}
	}
	return client, closeClient, nil
}

func authenticate(client *smtp.Client, config emailapp.SMTPConfig) error {
	if config.Username == "" {
		return nil
	}
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp authentication: %w", err)
	}
	return nil
}

func encodeMessage(config emailapp.SMTPConfig, recipient string, subject string, htmlBody string) ([]byte, error) {
	if err := validateHeader(subject, "subject"); err != nil {
		return nil, err
	}
	if err := validateHeader(config.FromName, "from name"); err != nil {
		return nil, err
	}
	var encodedBody bytes.Buffer
	quotedPrintable := quotedprintable.NewWriter(&encodedBody)
	if _, err := quotedPrintable.Write([]byte(htmlBody)); err != nil {
		return nil, fmt.Errorf("encode smtp body: %w", err)
	}
	if err := quotedPrintable.Close(); err != nil {
		return nil, fmt.Errorf("finish smtp body encoding: %w", err)
	}

	fromHeader := (&mail.Address{Name: config.FromName, Address: config.From}).String()
	toHeader := (&mail.Address{Address: recipient}).String()
	var message bytes.Buffer
	message.WriteString("From: " + fromHeader + "\r\n")
	message.WriteString("To: " + toHeader + "\r\n")
	message.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n")
	message.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	message.WriteString("\r\n")
	message.Write(encodedBody.Bytes())
	return message.Bytes(), nil
}

func envelopeAddress(value string, label string) (string, error) {
	if err := validateHeader(value, label); err != nil {
		return "", err
	}
	parsed, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || parsed.Name != "" {
		return "", fmt.Errorf("invalid smtp %s address", label)
	}
	return parsed.Address, nil
}

func validateHeader(value string, label string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("smtp %s contains a line break", label)
	}
	return nil
}

func connectionDeadline(ctx context.Context, timeout time.Duration) time.Time {
	deadline := time.Now().Add(timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		return contextDeadline
	}
	return deadline
}
