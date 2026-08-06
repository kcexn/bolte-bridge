package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"bolte-bridge/internal/store/sqlite"
)

// openInspector opens a second, raw connection to the database at path so a test
// can inspect committed state (schema version, table contents) independently of
// the Store under test.
func openInspector(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open inspector: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	return v
}

// TestOpenProvisions checks that Open creates the database file (including a
// missing parent directory) and provisions the schema.
func TestOpenProvisions(t *testing.T) {
	ctx := context.Background()
	// "sub" does not exist yet: Open must create it.
	path := filepath.Join(t.TempDir(), "sub", "bolte.db")

	s, err := Open(ctx, Config{SQLite: SQLiteConfig{Path: path}})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })

	db := openInspector(t, path)
	// After Open the schema is fully migrated, so user_version equals the number
	// of migrations.
	if want, got := sqlite.MigrationsCount(), userVersion(t, db); got != want {
		t.Errorf("user_version = %d, want %d", got, want)
	}
}

// TestOpenSQLiteEmptyPath checks that openSQLite returns an error when the
// path is empty.
func TestOpenSQLiteEmptyPath(t *testing.T) {
	ctx := context.Background()

	s, err := Open(ctx, Config{SQLite: SQLiteConfig{Path: ""}})
	if err == nil {
		_ = s.Close(ctx)
		t.Fatal("Open with empty path returned nil error, want failure")
	}
	if s != nil {
		t.Errorf("Open with empty path returned a non-nil Store %v, want nil", s)
	}
}

// TestOpenUnknownDriver checks that Open rejects a Driver it does not recognise
// rather than silently falling back to the SQLite backend.
func TestOpenUnknownDriver(t *testing.T) {
	ctx := context.Background()

	s, err := Open(ctx, Config{Driver: Driver("postgres")})
	if err == nil {
		_ = s.Close(ctx)
		t.Fatal("Open with an unknown driver returned nil error, want failure")
	}
	if s != nil {
		t.Errorf("Open with an unknown driver returned a non-nil Store %v, want nil", s)
	}
}

// TestSingleton checks that Init installs a store reachable via Client and that
// a repeat Init is a no-op that leaves the installed store unchanged.
func TestSingleton(t *testing.T) {
	ctx := context.Background()
	resetForTest(ctx)
	t.Cleanup(func() { resetForTest(ctx) })

	if err := Init(
		ctx,
		Config{SQLite: SQLiteConfig{Path: filepath.Join(t.TempDir(), "bolte.db")}},
	); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	first := Client()

	// A second Init is a no-op: the sync.Once guard keeps the store installed by
	// the first call, even when called with a different configuration.
	if err := Init(
		ctx,
		Config{SQLite: SQLiteConfig{Path: filepath.Join(t.TempDir(), "other.db")}},
	); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if Client() != first {
		t.Error("second Init changed the installed store; want it unchanged")
	}
}

// TestInitPropagatesOpenError checks that when Open fails, Init records the
// error and returns it, leaving no store installed for Client to hand out.
func TestInitPropagatesOpenError(t *testing.T) {
	ctx := context.Background()
	resetForTest(ctx)
	t.Cleanup(func() { resetForTest(ctx) })

	// An unknown driver makes Open fail, exercising the error branch in Init.
	if err := Init(ctx, Config{Driver: Driver("postgres")}); err == nil {
		t.Fatal("Init with an unknown driver returned nil error, want failure")
	}
	if client != nil {
		t.Errorf("Init installed a store %v after a failed Open, want none", client)
	}
}

// TestClientPanicsBeforeInit checks that reaching for the store before Init is a
// programmer error surfaced immediately.
func TestClientPanicsBeforeInit(t *testing.T) {
	ctx := context.Background()
	resetForTest(ctx)
	t.Cleanup(func() { resetForTest(ctx) })

	defer func() {
		if recover() == nil {
			t.Error("expected Client to panic before Init")
		}
	}()
	_ = Client()
}

// TestAdapterWithTx checks that the sqliteAdapter.WithTx correctly bridges
// between the sqlite subpackage and the store package interfaces.
func TestAdapterWithTx(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bolte.db")

	s, err := Open(ctx, Config{SQLite: SQLiteConfig{Path: path}})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })

	// Verify that WithTx executes the callback and that the Tx can be used.
	callbackExecuted := false
	if err := s.WithTx(ctx, func(ctx context.Context, tx Tx) error {
		callbackExecuted = true
		// Verify that we received a Tx and can use it to query the database.
		_, err := tx.Email().(*sqlite.TxEmail).Tx.ExecContext(ctx, "SELECT 1")
		return err
	}); err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	if !callbackExecuted {
		t.Error("callback was not executed")
	}
}

// TestAdapterWithTxRollback checks that the adapter rolls back the transaction
// when the callback returns an error.
func TestAdapterWithTxRollback(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bolte.db")

	s, err := Open(ctx, Config{SQLite: SQLiteConfig{Path: path}})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })

	// Create a test table in a successful transaction.
	if err := s.WithTx(ctx, func(ctx context.Context, tx Tx) error {
		_, err := tx.Email().(*sqlite.TxEmail).Tx.ExecContext(ctx, `
			CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)
		`)
		return err
	}); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Attempt to insert data but return an error to trigger rollback.
	testErr := errors.New("test error")
	err = s.WithTx(ctx, func(ctx context.Context, tx Tx) error {
		_, insertErr := tx.Email().(*sqlite.TxEmail).Tx.ExecContext(
			ctx,
			"INSERT INTO test (id, value) VALUES (1, 'should be rolled back')",
		)
		if insertErr != nil {
			return insertErr
		}
		return testErr
	})

	// Verify that the error was returned.
	if !errors.Is(err, testErr) {
		t.Errorf("WithTx returned %v, want %v", err, testErr)
	}

	// Verify that the data was rolled back by checking the table is empty.
	if err := s.WithTx(ctx, func(ctx context.Context, tx Tx) error {
		var count int
		return tx.Email().(*sqlite.TxEmail).Tx.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM test",
		).Scan(&count)
	}); err != nil {
		t.Fatalf("query count: %v", err)
	}
}
