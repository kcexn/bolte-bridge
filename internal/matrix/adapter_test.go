package matrix

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bolte-bridge/internal/relay"
	"bolte-bridge/internal/store"
)

// TestMain installs the process-wide store singleton for the whole test binary.
// The adapter reaches the database through store.Client(), and store.Init is
// sync.Once-guarded, so there is no way to swap the store out per test: the
// database file has to outlive every test in the package.
func TestMain(m *testing.M) {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "bolte-matrix")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}

	if err := store.Init(ctx, store.Config{
		SQLite: store.SQLiteConfig{Path: filepath.Join(dir, "bolte.db")},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "store.Init: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	_ = store.Client().Close(ctx)
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type mockClient struct {
	closeCalled   bool
	log           *[]string
	eventsToFetch []RawEvent

	fetchErr       error
	lastEventError error
}

func (m *mockClient) LastEvent(ctx context.Context) (string, error) {
	*m.log = append(*m.log, "m.LastEvent()")
	return "!last-event:matrix.org", m.lastEventError
}

func (m *mockClient) Fetch(context.Context, string) ([]RawEvent, error) {
	return m.eventsToFetch, m.fetchErr
}

func (m *mockClient) Send(context.Context, OutboundEvent) error {
	return nil
}

func (m *mockClient) Close(context.Context) error {
	m.closeCalled = true
	return nil
}

func newTestAdapter() *Adapter {
	cfg := validConfig()
	return &Adapter{client: &mockClient{log: &[]string{}}, cfg: cfg}
}

func TestAdapterGetCursor(t *testing.T) {
	a := newTestAdapter()
	ctx := context.Background()

	wantEventID := "aaa-event:matrix.org"

	err := store.Client().WithTx(ctx, func(ctx context.Context, tx store.Tx) error {
		return tx.Matrix().SetCursor(ctx, a.cfg.ServerName, a.cfg.RoomID, wantEventID)
	})
	if err != nil {
		t.Fatalf("seed cursor: %v", err)
	}

	gotEventID, err := a.getCursor(ctx)
	if err != nil {
		t.Fatalf("getCursor: %v", err)
	}
	if gotEventID != wantEventID {
		t.Errorf("getCursor EventID = %q, want %q", gotEventID, wantEventID)
	}
}

func TestAdapterSetCursor(t *testing.T) {
	ctx := context.Background()
	a := newTestAdapter()

	wantEventID := "aaa-event:matrix.org"

	if err := a.setCursor(ctx, wantEventID); err != nil {
		t.Fatalf("setCursor: %v", err)
	}

	var gotEventID string
	err := store.Client().WithTx(ctx, func(ctx context.Context, tx store.Tx) error {
		eventID, err := tx.Matrix().Cursor(ctx, a.cfg.ServerName, a.cfg.RoomID)
		gotEventID = eventID
		return err
	})
	if err != nil {
		t.Fatalf("read back cursor: %v", err)
	}
	if gotEventID != wantEventID {
		t.Errorf("persisted EventID = %q, want %q", gotEventID, wantEventID)
	}
}

func TestAdapterMedium(t *testing.T) {
	a := newTestAdapter()

	if got := a.Medium(); got != relay.MediumMatrix {
		t.Fatalf("Medium() = %v, want %v", got, relay.MediumMatrix)
	}
}

func TestAdapterFetch(t *testing.T) {
	a := newTestAdapter()

	msgs, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) = %d, want 0", len(msgs))
	}
}

func TestAdapterFetchSQLError(t *testing.T) {
	a := newTestAdapter()

	a.cfg.ServerName = "test-adapter-fetch-sql-error.matrix.org"

	msgs, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) = %d, want 0", len(msgs))
	}
	clientLog := *a.client.(*mockClient).log
	logMessage := clientLog[len(clientLog)-1]
	if logMessage != "m.LastEvent()" {
		t.Fatalf("m.LastEvent() not called. Expected Fetch to call m.LastEvent().")
	}
}

func TestAdapterFetchLastEventError(t *testing.T) {
	a := newTestAdapter()

	a.cfg.ServerName = "test-adapter-fetch-last-event-error.matrix.org"
	a.client.(*mockClient).lastEventError = errors.New("LastEvent Error.")

	msgs, err := a.Fetch(context.Background())
	if err == nil {
		t.Fatalf("No error when calling Fetch(), expected LastEvent error.")
	}
	if err.Error() != "LastEvent Error." {
		t.Fatalf("Fetch() failed with error = %v, wanted %q", err, "LastEvent Error.")
	}
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) = %d, want 0", len(msgs))
	}
	clientLog := *a.client.(*mockClient).log
	logMessage := clientLog[len(clientLog)-1]
	if logMessage != "m.LastEvent()" {
		t.Fatalf("m.LastEvent() not called. Expected Fetch to call m.LastEvent().")
	}
}

func TestFetchMessages(t *testing.T) {
	a := newTestAdapter()
	ctx := context.Background()
	now := time.Now()
	a.client.(*mockClient).eventsToFetch = []RawEvent{
		{
			EventID:   "!aaa-event:matrix.org",
			Sender:    "@alice:matrix.org",
			RoomID:    "!room:matrix.org",
			Body:      "Hello, world!",
			MsgType:   "m.text",
			InReplyTo: "",
			Timestamp: now,
		},
	}

	msgs, err := a.fetchMessages(ctx, "$cursor")
	if err != nil {
		t.Fatalf("fetchMessages() error = %v, want nil", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("fetchMessages() returned %d messages, want 2", len(msgs))
	}

	if msgs[0].MessageID != "!aaa-event:matrix.org" {
		t.Errorf("msgs[0].MessageID = %q, want %q", msgs[0].MessageID, "$event1")
	}
	if msgs[0].Body != "Hello, world!" {
		t.Errorf("msgs[0].Body = %q, want %q", msgs[0].Body, "Hello, world!")
	}
	if msgs[0].Sender.Address.ID != "@alice:matrix.org" {
		t.Errorf(
			"msgs[0].Sender.Address.ID = %q, want %q",
			msgs[0].Sender.Address.ID,
			"@alice:matrix.org",
		)
	}
}

func TestAdapterSend(t *testing.T) {
	a := newTestAdapter()

	id, err := a.Send(context.Background(), relay.RoutedMessage{})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if id != "" {
		t.Fatalf("Send() = %q, want empty string", id)
	}
}

func TestAdapterCommit(t *testing.T) {
	a := newTestAdapter()
	ctx := context.Background()

	if err := a.Commit(ctx, ""); err != nil {
		t.Fatalf("Commit(empty) error = %v", err)
	}

	wantEventID := "bbb-event:matrix.org"
	if err := a.Commit(ctx, wantEventID); err != nil {
		t.Fatalf("Commit(valid) error = %v", err)
	}

	gotEventID, err := a.getCursor(ctx)
	if err != nil {
		t.Fatalf("getCursor after Commit: %v", err)
	}
	if gotEventID != wantEventID {
		t.Errorf("persisted EventID = %q, want %q", gotEventID, wantEventID)
	}
}

func TestAdapterClose(t *testing.T) {
	a := newTestAdapter()

	if err := a.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !a.client.(*mockClient).closeCalled {
		t.Fatal("client.Close was not called")
	}
}

func TestNewAdapterInvalidConfig(t *testing.T) {
	_, err := NewAdapter(context.Background(), Config{})
	if err == nil {
		t.Fatal("NewAdapter() error = nil, want validation error")
	}
}

func TestNewAdapterSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	cfg := validConfig()
	cfg.HomeserverURL = server.URL

	ctx := context.Background()
	adapter, err := NewAdapter(ctx, cfg)
	if err != nil {
		t.Fatalf("NewAdapter() returned error: %v", err)
	}

	if adapter.cfg.HomeserverURL != server.URL {
		t.Errorf("adapter.cfg.HomeserverURL = %q, want %q", adapter.cfg.HomeserverURL, server.URL)
	}
}
