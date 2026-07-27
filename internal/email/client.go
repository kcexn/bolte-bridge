// Package email provides the bridge's Email Adapter.
// Authentication is username + password, carried in Config.
package email

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
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

// MailboxInfo holds metadata about a selected mailbox.
type MailboxInfo struct {
	Client     *imapclient.Client
	SelectData *imap.SelectData
}

// Client is the transport surface of the Email Adapter. Its methods each open a
// fresh connection, do their work, and close it (see New).
type Client interface {
	// LatestUID returns the current UIDNext and UIDVaildity values of the
	// configured mailbox.
	LatestUID() (uint32, uint32)

	// Fetch returns every message in the configured mailbox with a UID greater
	// than sinceUID, oldest (lowest UID) first. A sinceUID of 0 returns the whole
	// mailbox. Advancing the sinceUID cursor remains the caller's
	// job, not a side effect of Fetch.
	Fetch(ctx context.Context, sinceUID uint32) ([]RawMessage, error)

	// Send submits one pre-built RFC 822 message over SMTP. from is the envelope
	// sender (MAIL FROM) and to the envelope recipients (RCPT TO); raw carries the
	// message headers and body. Envelope addresses are passed explicitly.
	Send(ctx context.Context, from string, to []string, raw []byte) error

	// Safely close the email client.
	Close(ctx context.Context) error
}

// NewClient constructs a Client for the given account.
func NewClient(ctx context.Context, cfg Config) (Client, error) {
	return newEmailClient(ctx, cfg)
}

// emailClient is the IMAP/SMTP-over-TLS implementation of Client. It holds only
// validated configuration; every operation dials its own connection.
type emailClient struct {
	cfg  Config
	mbox *MailboxInfo
}

// Compile-time assertion that emailClient satisfies Client.
var _ Client = (*emailClient)(nil)

// newEmailClient validates cfg and returns a ready emailClient.
func newEmailClient(ctx context.Context, cfg Config) (*emailClient, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	mbox, err := dialAndSelectMailbox(cfg)
	if err != nil {
		return nil, err
	}

	return &emailClient{
		cfg:  cfg,
		mbox: mbox,
	}, nil
}

// LatestUID returns the current UIDNext and UIDValidity values of the
// configured mailbox.
func (c *emailClient) LatestUID() (uint32, uint32) {
	return uint32(c.mbox.SelectData.UIDNext), c.mbox.SelectData.UIDValidity
}

// Close logs out of IMAP and closes the connection.
func (c *emailClient) Close(ctx context.Context) error {
	if c.mbox == nil || c.mbox.Client == nil {
		return nil
	}
	if err := c.mbox.Client.Logout().Wait(); err != nil {
		_ = c.mbox.Client.Close()
		return fmt.Errorf("email: IMAP logout: %w", err)
	}
	return c.mbox.Client.Close()
}

// validateConfig checks that all required fields in cfg are populated.
func validateConfig(cfg Config) error {
	if cfg.Username == "" {
		return errors.New("email: Config.Username is required")
	}
	if cfg.Password == "" {
		return errors.New("email: Config.Password is required")
	}
	if cfg.IMAPAddr == "" {
		return errors.New("email: Config.IMAPAddr is required")
	}
	if cfg.SMTPAddr == "" {
		return errors.New("email: Config.SMTPAddr is required")
	}
	if cfg.Mailbox == "" {
		return errors.New("email: Config.Mailbox is required")
	}
	return nil
}

// dialAndSelectMailbox connects to the IMAP server, logs in, and selects the mailbox.
func dialAndSelectMailbox(cfg Config) (*MailboxInfo, error) {
	client, err := imapclient.DialTLS(cfg.IMAPAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("email: dial IMAP %s: %w", cfg.IMAPAddr, err)
	}

	if err := client.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("email: IMAP login: %w", err)
	}

	mbox, err := client.Select(cfg.Mailbox, nil).Wait()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("email: select mailbox %q: %w", cfg.Mailbox, err)
	}

	return &MailboxInfo{
		Client:     client,
		SelectData: mbox,
	}, nil
}
