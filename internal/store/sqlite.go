package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	// modernc.org/sqlite is a pure-Go (cgo-free) SQLite driver. It registers
	// itself with database/sql under the name "sqlite" and keeps
	// CGO_ENABLED=0 cross-compilation working for every release target.
	_ "modernc.org/sqlite"
)

// SQLiteConfig parameterises how the SQLite backend is opened and provisioned.
// It is referenced from Config.SQLite in store.go.
type SQLiteConfig struct {
	// Path is the filesystem path to the SQLite database file. Its parent
	// directory is created if it does not exist, and the file itself is created
	// on first open. Required for the SQLite backend.
	Path string
}

//go:embed schemaV1.sql
var schemaV1 string

// migrations is the ordered list of schema steps. Index i is the migration that
// advances PRAGMA user_version from i to i+1; new steps are appended and never
// edited once released.
var migrations = []string{
	schemaV1,
}

// openSQLite opens (creating and provisioning if necessary) a SQLite-backed
// Store at cfg.Path. It creates the parent directory and database file if
// missing and brings the schema up to date by running any pending migrations.
// The returned Store is safe to use immediately. It is reached through the
// backend dispatch in Open.
func openSQLite(ctx context.Context, cfg SQLiteConfig) (Store, error) {
	if cfg.Path == "" {
		return nil, errors.New("store: SQLiteConfig.Path is required")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o750); err != nil {
		return nil, fmt.Errorf("store: create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dsn(cfg))
	if err != nil {
		return nil, fmt.Errorf("store: open database: %w", err)
	}
	// A one-shot CLI has no concurrent access; a single connection sidesteps
	// SQLite writer-lock contention and keeps connection pragmas consistent.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: connect to database: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: apply migrations: %w", err)
	}
	return &sqliteStore{db: db}, nil
}

// dsn builds the modernc.org/sqlite connection string, applying connection
// pragmas via the driver's ?_pragma= query parameters.
func dsn(cfg SQLiteConfig) string {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(on)")
	q.Add("_pragma", "journal_mode(WAL)")
	return "file:" + cfg.Path + "?" + q.Encode()
}

// migrate brings the database schema up to the latest version. It reads the
// current PRAGMA user_version and applies each pending migration in its own
// transaction, bumping user_version as it goes. It is idempotent: on an
// already-current database it applies nothing.
func migrate(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	for i := version; i < len(migrations); i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("run migration %d: %w", i+1, err)
		}
		// PRAGMA statements cannot be parameterised; the value is an integer we
		// control, so interpolation is safe here.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("bump schema version to %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", i+1, err)
		}
	}
	return nil
}

// sqliteStore is the SQLite-backed implementation of Store.
type sqliteStore struct {
	db *sql.DB
}

func (s *sqliteStore) WithTx(ctx context.Context, fn func(context.Context, Tx) error) (err error) {
	sqlTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = sqlTx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = sqlTx.Rollback()
		}
	}()

	if err = fn(ctx, &sqliteTx{tx: sqlTx}); err != nil {
		return err
	}
	if err = sqlTx.Commit(); err != nil {
		return fmt.Errorf("store: commit transaction: %w", err)
	}
	return nil
}

func (s *sqliteStore) Close(_ context.Context) error {
	return s.db.Close()
}

// sqliteTx is the SQLite-backed implementation of Tx. It wraps the live *sql.Tx
// handed to it by WithTx; its methods run their SQL directly against that
// transaction.
type sqliteTx struct {
	tx *sql.Tx
}

func (t *sqliteTx) SmokeTest(ctx context.Context) (int, error) {
	var n int
	if err := t.tx.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
		return 0, fmt.Errorf("store: smoke test: %w", err)
	}
	return n, nil
}

// Compile-time assertion that these types satisfy the store interfaces.
var (
	_ Store = (*sqliteStore)(nil)
	_ Tx    = (*sqliteTx)(nil)
)
