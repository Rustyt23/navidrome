package gcpsync

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

type syncReport struct {
	Operation string
	Uploaded  []string
	Failed    []string
}

func (r syncReport) body() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\nBucket: %s\nUploaded: %d\nFailed: %d\n", r.Operation, conf.Server.GCPSync.Bucket, len(r.Uploaded), len(r.Failed))
	for _, name := range r.Uploaded {
		fmt.Fprintf(&b, "\nUploaded: %s", name)
	}
	for _, failure := range r.Failed {
		fmt.Fprintf(&b, "\nFailed: %s", failure)
	}
	if len(r.Failed) > 0 {
		b.WriteString("\n\nCheck the reported files and retry Sync. Files that could not be uploaded remain staged.")
	}
	return b.String()
}

func (s *service) deliverReport(ctx context.Context, report syncReport) {
	if !conf.Server.GCPSync.Notifications.Enabled && s.notify == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	send := s.notify
	if send == nil {
		send = sendSyncMail
	}
	if err := send(ctx, report); err != nil {
		log.Warn(ctx, "GCPSync: notification email failed", err)
	}
}

func notificationMessage(from, to, subject, body string) (string, string, []byte, error) {
	if strings.ContainsAny(from+to+subject, "\r\n") {
		return "", "", nil, fmt.Errorf("email headers must not contain line breaks")
	}
	sender, err := mail.ParseAddress(from)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid notification sender: %w", err)
	}
	recipient, err := mail.ParseAddress(to)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid notification recipient: %w", err)
	}
	message := "From: " + sender.String() + "\r\nTo: " + recipient.String() + "\r\nSubject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n") + "\r\n"
	return sender.Address, recipient.Address, []byte(message), nil
}

func sendSyncMail(ctx context.Context, report syncReport) error {
	opts := conf.Server.GCPSync.Notifications
	from := opts.From
	if from == "" {
		from = opts.Username
	}
	subject := opts.Subject
	if subject == "" {
		subject = "Music sync result"
	}
	sender, recipient, message, err := notificationMessage(from, opts.To, subject, report.body())
	if err != nil {
		return err
	}
	host := strings.TrimSpace(opts.SMTPServer)
	if host == "" {
		return fmt.Errorf("GCPSync.Notifications.SMTPServer is required")
	}
	port := opts.SMTPPort
	if port == 0 {
		port = 587
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	security := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var connection net.Conn
	if port == 465 {
		connection, err = (&tls.Dialer{NetDialer: dialer, Config: security}).DialContext(ctx, "tcp", address)
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return err
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if port != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not support required STARTTLS")
		}
		if err := client.StartTLS(security); err != nil {
			return err
		}
	}
	if opts.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", opts.Username, opts.Password, host)); err != nil {
			return err
		}
	}
	if err := client.Mail(sender); err != nil {
		return err
	}
	if err := client.Rcpt(recipient); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
