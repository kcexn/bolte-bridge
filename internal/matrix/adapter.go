package matrix

import (
	"context"

	"bolte-bridge/internal/core"
	"bolte-bridge/internal/relay"
	"bolte-bridge/internal/store"
)

// Adapter is the Matrix medium edge of the bridge.
type Adapter struct {
	client Client
	cfg    Config
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

// Commit will durably advance the Matrix EventID cursor. An empty cursor
// commits everything returned by the preceding Fetch.
func (a *Adapter) Commit(ctx context.Context, cursor string) error {
	if cursor == "" {
		return nil
	}

	// Persist the cursor within a database transaction.
	return store.Client().WithTx(ctx, func(txCtx context.Context, tx store.Tx) error {
		return tx.Matrix().SetCursor(txCtx, a.cfg.ServerName, a.cfg.RoomID, cursor)
	})
}

// Close closes the underlying Matrix client.
func (a *Adapter) Close(ctx context.Context) error {
	return a.client.Close(ctx)
}
