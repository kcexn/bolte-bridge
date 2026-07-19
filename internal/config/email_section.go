package config

import (
	"errors"

	"bolte-bridge/internal/email"
)

// Email endpoint defaults, used when the corresponding flag/env is unset. They
// target Gmail: implicit-TLS IMAP on 993, STARTTLS SMTP submission on 587.
const (
	defaultIMAPAddr = "imap.gmail.com:993"
	defaultSMTPAddr = "smtp.gmail.com:587"
	defaultMailbox  = "INBOX"
)

// emailSection configures the email transport client. Username
// and endpoints take flags with Gmail-friendly defaults.
func emailSection(b *Binder) (ApplyFunc, error) {
	b.StringP(
		"email.account",
		"email",
		"e",
		"",
		"account name for IMAP/SMTP (the full email address).",
	)
	b.Secret("email.password", "")
	b.String(
		"email.imap-addr",
		"email-imap-addr",
		defaultIMAPAddr,
		"host:port of the IMAP endpoint (implicit TLS).",
	)
	b.String(
		"email.smtp-addr",
		"email-smtp-addr",
		defaultSMTPAddr,
		"host:port of the SMTP submission endpoint (STARTTLS).",
	)
	b.String("email.mailbox", "email-mailbox", defaultMailbox, "IMAP mailbox to fetch from.")

	return func(cfg *Config) error {
		v := b.Viper()
		username := v.GetString("email.account")
		if username == "" {
			return errors.New("email.account must not be empty")
		}
		password := v.GetString("email.password")
		if password == "" {
			return errors.New("email.password must not be empty (set BOLTE_BRIDGE_EMAIL_PASSWORD)")
		}
		cfg.Email = email.Config{
			Username: username,
			Password: password,
			IMAPAddr: v.GetString("email.imap-addr"),
			SMTPAddr: v.GetString("email.smtp-addr"),
			Mailbox:  v.GetString("email.mailbox"),
		}
		return nil
	}, nil
}
