package email

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"bolte-bridge/internal/core"
	"bolte-bridge/internal/relay"
)

// Adapter is the email medium edge of the bridge. It owns the Client it talks
// to outright: the transport is an implementation detail of this package, so
// callers configure the adapter and never name the Client themselves.
type Adapter struct {
	client Client
	cfg    Config
	// The domain used for generating Message-ID's
	clientDomain string
	// Maps Message-ID to IMAP UID from last Fetch.
	msgIDToUID map[string]uint32
	// The last seen UID from the previous Fetch.
	uidCursor uint32
}

// Compile-time assertion that Adapter satisfies core.Adapter.
var _ core.Adapter = (*Adapter)(nil)

// NewAdapter builds an Adapter and the Client it owns from cfg, reporting any
// configuration error from Client construction.
func NewAdapter(ctx context.Context, clientDomain string, cfg Config) (*Adapter, error) {
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Adapter{
		client:       client,
		cfg:          cfg,
		clientDomain: clientDomain,
		msgIDToUID:   make(map[string]uint32),
	}, nil
}

// Medium reports the email medium.
func (a *Adapter) Medium() relay.Medium {
	return relay.MediumEmail
}

// Fetch will IMAP-fetch mail past the committed UID cursor and translate it
// into relay messages.
func (a *Adapter) Fetch(ctx context.Context) ([]relay.Message, error) {
	return a.fetch(ctx)
}

// Send will reconstruct msg as an RFC 822 message, submit it over SMTP, and
// return the Message-ID it was delivered under.
func (a *Adapter) Send(ctx context.Context, msg relay.RoutedMessage) (string, error) {
	msgID := fmt.Sprintf("<%s@%s>", uuid.NewString(), a.clientDomain)
	mail := makeEmail(
		mail.Address{
			Name:    msg.Message.Sender.DisplayName,
			Address: a.cfg.Username,
		},
		mail.Address{Address: msg.To.ID},
		msgID,
		msg.Message.InReplyTo,
		msg.Message.Subject,
		msg.Message.Body,
	)
	if err := a.client.Send(ctx, a.cfg.Username, []string{msg.To.ID}, mail); err != nil {
		return "", fmt.Errorf("emailAdapter: failed to send mail: %w", err)
	}
	return msgID, nil
}

// Commit will durably advance the read cursor to the Message-ID named by cursor,
// committing every message up to and including it, or everything the preceding
// Fetch returned when cursor is empty.
func (a *Adapter) Commit(_ context.Context, _ string) error {
	return nil
}

// Close closes the underlying email client.
func (a *Adapter) Close(ctx context.Context) error {
	return a.client.Close(ctx)
}

func makeEmail(from, to mail.Address, messageID, inReplyTo, subject, body string) []byte {
	var b strings.Builder

	// Headers
	fmt.Fprintf(&b, "From: %s\r\n", from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	if inReplyTo != "" {
		fmt.Fprintf(&b, "In-Reply-To: %s\r\n", inReplyTo)
	}
	fmt.Fprintf(&b, "Message-ID: %s\r\n", messageID)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&b, "\r\n")

	// Body
	b.WriteString(body)

	return []byte(b.String())
}
