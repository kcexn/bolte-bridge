package matrix

import (
	"context"
	"fmt"

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
func (a *Adapter) Send(_ context.Context, _ relay.RoutedMessage) (string, error) {
	return "", nil
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
