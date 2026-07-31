package store

import (
	"context"

	"bolte-bridge/internal/store/sqlite"
)

// SQLiteConfig parameterises how the SQLite backend is opened and provisioned.
// It is referenced from Config.SQLite in store.go.
type SQLiteConfig struct {
	// Path is the filesystem path to the SQLite database file. Its parent
	// directory is created if it does not exist, and the file itself is created
	// on first open. Required for the SQLite backend.
	Path string
}

// sqliteAdapter wraps the sqlite.Store to implement the Store interface.
type sqliteAdapter struct {
	store *sqlite.Store
}

func (a *sqliteAdapter) WithTx(
	ctx context.Context,
	fn func(context.Context, Tx) error,
) error {
	return a.store.WithTx(ctx, func(ctx context.Context, tx *sqlite.Tx) error {
		return fn(ctx, &sqliteTxAdapter{tx: tx})
	})
}

func (a *sqliteAdapter) Close(ctx context.Context) error {
	return a.store.Close(ctx)
}

// sqliteTxAdapter wraps sqlite.Tx to implement the Tx interface.
type sqliteTxAdapter struct {
	tx *sqlite.Tx
}

func (a *sqliteTxAdapter) Email() TxEmail {
	return a.tx.Email
}

func (a *sqliteTxAdapter) Matrix() TxMatrix {
	return a.tx.Matrix
}

// openSQLite opens (creating and provisioning if necessary) a SQLite-backed
// Store at cfg.Path. It creates the parent directory and database file if
// missing and brings the schema up to date by running any pending migrations.
// The returned Store is safe to use immediately. It is reached through the
// backend dispatch in Open.
func openSQLite(ctx context.Context, cfg SQLiteConfig) (Store, error) {
	s, err := sqlite.Open(ctx, sqlite.Config{Path: cfg.Path})
	if err != nil {
		return nil, err
	}
	return &sqliteAdapter{store: s}, nil
}

// Compile-time assertions.
var (
	_ Store = (*sqliteAdapter)(nil)
	_ Tx    = (*sqliteTxAdapter)(nil)
)
