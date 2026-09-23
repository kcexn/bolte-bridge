package matrix

import (
	"context"
	"fmt"
	"strings"

	"bolte-bridge/internal/core"
	"bolte-bridge/internal/relay"
	"bolte-bridge/internal/store"
)

// Adapter is the Matrix medium edge of the bridge.
type Adapter struct {
	client Client
	cfg    Config
	// Set of Matrix EventID's seen in the previous Fetch.
	lastFetched map[string]bool
	// The last seen EventID from the previous Fetch.
	lastEventID string
}

// Compile-time assertion that Adapter satisfies core.Adapter.
var _ core.Adapter = (*Adapter)(nil)

// NewAdapter builds an Adapter and the Client it owns from ctx and cfg,
// reporting any configuration or initialization error from Client construction.
func NewAdapter(ctx context.Context, cfg Config) (*Adapter, error) {
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &Adapter{
		client: client,
		cfg:    cfg,
	}, nil
}

// Medium reports the Matrix medium.
func (a *Adapter) Medium() relay.Medium {
	return relay.MediumMatrix
}

// Fetch will fetch Matrix events past the committed EventID and translate
// them into relay messages, advancing only the in-memory cursor.
func (a *Adapter) Fetch(ctx context.Context) ([]relay.Message, error) {
	return a.fetch(ctx)
}

// Send will translate a routed message into a Matrix event, send it to the
// configured room, and return the EventID assigned by the homeserver.
func (a *Adapter) Send(ctx context.Context, msg relay.RoutedMessage) (string, error) {
	senderID, ok := ghostSenderID(msg.Message.Sender, a.cfg)
	if !ok {
		return "", nil
	}

	outboundMsg := OutboundEvent{
		RoomID:      a.cfg.RoomID,
		Sender:      senderID,
		DisplayName: msg.Message.Sender.DisplayName,
		ReplyTo:     msg.Message.InReplyTo,
		Body:        msg.Message.Body,
	}

	eventID, err := a.client.Send(ctx, outboundMsg)
	if err != nil {
		return "", fmt.Errorf("matrix: failed to send: %w", err)
	}

	return eventID, nil
}

// ghostSenderID constructs the Matrix ghost user ID for an inbound email sender.
// If the sender address is empty, not email, or not of the form localpart@domain,
// it reports ok=false indicating the message should be dropped.
func ghostSenderID(sender relay.Identity, cfg Config) (string, bool) {
	if sender.Address.Mode != relay.MediumEmail || sender.Address.ID == "" {
		return "", false
	}

	localpart, domain, ok := strings.Cut(sender.Address.ID, "@")
	if !ok || localpart == "" || domain == "" || strings.Contains(domain, "@") {
		return "", false
	}

	return fmt.Sprintf("@%s/%s/%s:%s", cfg.SenderLocalpart, domain, localpart, cfg.ServerName), true
}

// Commit will durably advance the Matrix EventID cursor.
// An empty cursor commits everything returned by the preceding Fetch.
func (a *Adapter) Commit(ctx context.Context, cursor string) error {
	if cursor == "" {
		return a.setCursor(ctx, a.lastEventID)
	}

	if _, ok := a.lastFetched[cursor]; ok {
		return a.setCursor(ctx, cursor)
	}

	return fmt.Errorf(
		"matrix: failed to commit: cursor %q not found in fetched events",
		cursor,
	)
}

// Close closes the underlying Matrix client.
func (a *Adapter) Close(ctx context.Context) error {
	return a.client.Close(ctx)
}

// setCursor retrieves the current EventID from the store.
func (a *Adapter) setCursor(ctx context.Context, eventID string) error {
	return store.Client().WithTx(ctx, func(ctx context.Context, tx store.Tx) error {
		return tx.Matrix().SetCursor(ctx, a.cfg.ServerName, a.cfg.RoomID, eventID)
	})
}
