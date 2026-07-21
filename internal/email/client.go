// Package email provides the bridge's Email Adapter.
// Authentication is username + password, carried in Config.
package email

import (
	"context"
	"errors"
	"time"
)

// Config holds the account and endpoint settings for a Client. Values are
// resolved by the configuration factory (see internal/config); this package
// applies no defaults of its own.
type Config struct {
	// Username is the account login.
	Username string

	// Password is the account password.
	Password string

	// IMAPAddr is the "host:port" of the IMAP endpoint, reached over implicit
	// TLS (for Gmail, "imap.gmail.com:993").
	IMAPAddr string

	// SMTPAddr is the "host:port" of the SMTP submission endpoint, reached over
	// STARTTLS (for Gmail, "smtp.gmail.com:587").
	SMTPAddr string

	// Mailbox is the IMAP mailbox Fetch reads from (for Gmail, "INBOX").
	Mailbox string
}

// RawMessage is a single fetched message.
type RawMessage struct {
	// UID is the message's IMAP unique identifier within the mailbox. UIDs are
	// monotonically increasing for a given UIDValidity, so the caller advances
	// its read cursor by remembering the highest UID it has processed.
	UID uint32

	// UIDValidity is the mailbox's UIDVALIDITY at fetch time. If it changes
	// between runs, previously stored UIDs are meaningless and the cursor must
	// be reset — the caller must persist it alongside the UID cursor.
	UIDValidity uint32

	// InternalDate is the server's arrival timestamp for the message.
	InternalDate time.Time

	// Raw is the complete RFC 822 message (headers and body, i.e. IMAP BODY[]).
	Raw []byte
}

// Client is the transport surface of the Email Adapter. Its methods each open a
// fresh connection, do their work, and close it (see New).
type Client interface {
	// Fetch returns every message in the configured mailbox with a UID greater
	// than sinceUID, oldest (lowest UID) first. A sinceUID of 0 returns the whole
	// mailbox. Advancing the sinceUID cursor remains the caller's
	// job, not a side effect of Fetch.
	Fetch(ctx context.Context, sinceUID uint32) ([]RawMessage, error)

	// Send submits one pre-built RFC 822 message over SMTP. from is the envelope
	// sender (MAIL FROM) and to the envelope recipients (RCPT TO); raw carries the
	// message headers and body. Envelope addresses are passed explicitly.
	Send(ctx context.Context, from string, to []string, raw []byte) error
}

// NewClient constructs a Client for the given account.
func NewClient(cfg Config) (Client, error) {
	return newEmailClient(cfg)
}

// emailClient is the IMAP/SMTP-over-TLS implementation of Client. It holds only
// validated configuration; every operation dials its own connection.
type emailClient struct {
	cfg Config
}

// newEmailClient validates cfg and returns a ready emailClient.
func newEmailClient(cfg Config) (*emailClient, error) {
	if cfg.Username == "" {
		return nil, errors.New("email: Config.Username is required")
	}
	if cfg.Password == "" {
		return nil, errors.New("email: Config.Password is required")
	}
	if cfg.IMAPAddr == "" {
		return nil, errors.New("email: Config.IMAPAddr is required")
	}
	if cfg.SMTPAddr == "" {
		return nil, errors.New("email: Config.SMTPAddr is required")
	}
	if cfg.Mailbox == "" {
		return nil, errors.New("email: Config.Mailbox is required")
	}
	return &emailClient{cfg: cfg}, nil
}

// Compile-time assertion that emailClient satisfies Client.
var _ Client = (*emailClient)(nil)
