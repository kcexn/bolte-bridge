// Package store defines the bridge's state-store client and its SQLite
// implementation. Every component reaches the database through the medium-
// agnostic Store interface, obtained from the process-wide singleton installed
// by Init. The interface is a domain repository: callers invoke named methods on
// a transaction and never write SQL themselves.
package store

import (
	"context"
	"fmt"
	"sync"
)

// Store is the medium-agnostic database client. It owns the connection lifecycle
// and hands out transactions; it deliberately exposes no direct data-access
// methods, so that every read and write happens inside a WithTx scope.
type Store interface {
	// WithTx runs fn inside a single database transaction. The transaction is
	// committed if fn returns nil and rolled back if fn returns an error or
	// panics (the panic is then re-raised). The Tx passed to fn is valid only
	// for the duration of the call and must not be retained.
	WithTx(ctx context.Context, fn func(context.Context, Tx) error) error

	// Close releases the underlying database resources. It is called once, by
	// the owner of the store (main), at the end of a run.
	Close(ctx context.Context) error
}

// Tx is the transactional surface of the domain repository. Each method runs its
// SQL against the transaction it belongs to; callers obtain a Tx from
// Store.WithTx and never construct one. Domain methods are added here
// as they are needed.
type Tx interface {
	// SmokeTest is a placeholder domain method used to prove the
	// Store → Tx → SQL wiring end to end and to demonstrate the repository
	// pattern: the caller invokes a named method and the SQL stays inside it.
	// It executes `SELECT 1` on the transaction and returns 1. It carries no
	// business meaning and is expected to be removed once real domain methods
	// exist.
	SmokeTest(ctx context.Context) (int, error)
}

// Driver identifies which database backend backs a Store.
type Driver string

const (
	// DriverSQLite is the SQLite backend. It is the default when Config.Driver
	// is empty, and currently the only implemented backend.
	DriverSQLite Driver = "sqlite"
)

// Config selects and parameterises the database backend. Backend-specific
// settings live in their own nested block (e.g. SQLite), so switching databases
// is a configuration change. To add a backend: add a Driver constant,
// a nested config block, and a case in Open — no caller of
// Store or Client needs to change.
type Config struct {
	// Driver selects the backend. When empty it defaults to DriverSQLite.
	Driver Driver

	// SQLite holds SQLite-specific settings; used when Driver is DriverSQLite.
	// Its type is defined alongside the SQLite backend in sqlite.go.
	SQLite SQLiteConfig
}

// Open constructs a Store for the backend selected by cfg.Driver, provisioning
// it if necessary.
func Open(ctx context.Context, cfg Config) (Store, error) {
	switch cfg.Driver {
	case "", DriverSQLite:
		return openSQLite(ctx, cfg.SQLite)
	default:
		return nil, fmt.Errorf("store: unknown driver %q", cfg.Driver)
	}
}

// The process-wide singleton, installed once by Init and read by Client.
var (
	once    sync.Once
	client  Store
	initErr error
)

// Init constructs the store for the backend selected by cfg (via Open) and
// installs it as the process-wide singleton. main calls it once at startup;
// components then reach the store through Client.
func Init(ctx context.Context, cfg Config) error {
	once.Do(func() {
		s, err := Open(ctx, cfg)
		if err != nil {
			initErr = err
			return
		}
		client = s
	})
	return initErr
}

// Client returns the installed singleton store. It panics if Init has not
// completed successfully.
func Client() Store {
	if client == nil {
		panic("store: Client called before a successful Init")
	}
	return client
}

// resetForTest tears down the singleton so tests can install a fresh one. It is
// unexported and intended only for use by tests in this package.
func resetForTest(ctx context.Context) {
	if client != nil {
		_ = client.Close(ctx)
	}
	once = sync.Once{}
	client = nil
	initErr = nil
}
