package email

import (
	"context"

	"bolte-bridge/internal/core"
	"bolte-bridge/internal/relay"
)

// Adapter is the email medium edge of the bridge. It owns the Client it talks
// to outright: the transport is an implementation detail of this package, so
// callers configure the adapter and never name the Client themselves.
type Adapter struct {
	client Client
	cfg    Config
	// Maps Message-ID to IMAP UID from last Fetch.
	msgIDToUID map[string]uint32
}

// Compile-time assertion that Adapter satisfies core.Adapter.
var _ core.Adapter = (*Adapter)(nil)

// NewAdapter builds an Adapter and the Client it owns from cfg, reporting any
// configuration error from Client construction.
func NewAdapter(ctx context.Context, cfg Config) (*Adapter, error) {
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Adapter{client: client, cfg: cfg, msgIDToUID: make(map[string]uint32)}, nil
}

// Medium reports the email medium.
func (a *Adapter) Medium() relay.Medium {
	return relay.MediumEmail
}

// Fetch will IMAP-fetch mail past the committed UID cursor and translate it
// into relay messages, advancing only the in-memory cursor.
func (a *Adapter) Fetch(ctx context.Context) ([]relay.Message, error) {
	return a.fetch(ctx)
}

// Send will reconstruct msg as an RFC 822 message, submit it over SMTP, and
// return the Message-ID it was delivered under.
func (a *Adapter) Send(_ context.Context, _ relay.RoutedMessage) (string, error) {
	return "", nil
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
