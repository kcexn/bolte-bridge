package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
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

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var found string
	err := db.QueryRow(
		"SELECT name FROM sqlite_schema WHERE type='table' AND name=?", name,
	).Scan(&found)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false
	case err != nil:
		t.Fatalf("query sqlite_schema: %v", err)
	}
	return found == name
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
	if want, got := len(migrations), userVersion(t, db); got != want {
		t.Errorf("user_version = %d, want %d", got, want)
	}
	if !tableExists(t, db, "bridge_meta") {
		t.Error("bridge_meta table was not created")
	}
}

// TestWithTxSmokeTest is an example of the domain-repository usage pattern:
// obtain a Tx from WithTx and call a named domain method on it.
func TestWithTxSmokeTest(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, Config{SQLite: SQLiteConfig{Path: filepath.Join(t.TempDir(), "bolte.db")}})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })

	var got int
	err = s.WithTx(ctx, func(ctx context.Context, tx Tx) error {
		n, err := tx.SmokeTest(ctx)
		got = n
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if got != 1 {
		t.Errorf("SmokeTest returned %d, want 1", got)
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
