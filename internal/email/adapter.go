package email

import (
	"context"

	"bolte-bridge/internal/core"
	"bolte-bridge/internal/relay"
)

// Adapter is the email medium edge of the bridge. It owns the Client it talks
// to outright: the transport is an implementation detail of this package, so
// callers configure the adapter and never name the Client themselves.
//
// Its Fetch, Send, and Commit methods are not implemented yet.
type Adapter struct {
	client Client
}

// NewAdapter builds an Adapter and the Client it owns from cfg, reporting any
// configuration error from Client construction.
func NewAdapter(cfg Config) (*Adapter, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Adapter{client: client}, nil
}

// Medium reports the email medium.
func (a *Adapter) Medium() relay.Medium {
	return relay.MediumEmail
}

// Fetch will IMAP-fetch mail past the committed UID cursor and translate it
// into relay messages, advancing only the in-memory cursor.
func (a *Adapter) Fetch(_ context.Context) ([]relay.Message, error) {
	return nil, nil
}

// Send will reconstruct msg as an RFC 822 message, submit it over SMTP, and
// return the Message-ID it was delivered under.
func (a *Adapter) Send(_ context.Context, _ relay.RoutedMessage) (string, error) {
	return "", nil
}

// Commit will durably advance the read cursor to the IMAP UID named by cursor,
// committing every message up to and including it, or everything the preceding
// Fetch returned when cursor is empty.
func (a *Adapter) Commit(_ context.Context, _ string) error {
	return nil
}

// Compile-time assertion that Adapter satisfies core.Adapter.
var _ core.Adapter = (*Adapter)(nil)
