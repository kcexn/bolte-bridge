package matrix

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	sendEventID string
	sendErr     error
	sentEvents  []OutboundEvent
}

func (m *mockClient) LastEvent(ctx context.Context) (string, error) {
	*m.log = append(*m.log, "m.LastEvent()")
	return "!last-event:matrix.org", m.lastEventError
}

func (m *mockClient) Fetch(context.Context, string) ([]RawEvent, error) {
	return m.eventsToFetch, m.fetchErr
}

func (m *mockClient) Send(_ context.Context, msg OutboundEvent) (string, error) {
	m.sentEvents = append(m.sentEvents, msg)
	return m.sendEventID, m.sendErr
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
		t.Fatalf("fetchMessages() returned %d messages, want 1", len(msgs))
	}

	if msgs[0].MessageID != "!aaa-event:matrix.org" {
		t.Errorf("msgs[0].MessageID = %q, want %q", msgs[0].MessageID, "!aaa-event:matrix.org")
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

func TestFetchMessages_Empty(t *testing.T) {
	a := newTestAdapter()
	ctx := context.Background()
	a.client.(*mockClient).eventsToFetch = []RawEvent{}

	msgs, err := a.fetchMessages(ctx, "$cursor")
	if err != nil {
		t.Fatalf("fetchMessages() error = %v, want nil", err)
	}

	if len(msgs) != 0 {
		t.Fatalf("fetchMessages() returned %d messages, want 0", len(msgs))
	}

	if a.lastEventID != "" {
		t.Errorf("lastEventID = %q, want %q", a.lastEventID, "")
	}

	if a.lastFetched != nil {
		t.Errorf("lastFetched = %v, want nil", a.lastFetched)
	}
}

func TestGhostSenderID(t *testing.T) {
	cfg := validConfig()

	tests := []struct {
		name   string
		sender relay.Identity
		wantID string
		wantOK bool
	}{
		{
			name: "standard email",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumEmail, ID: "alice@example.com"},
			},
			wantID: "@bolte/example.com/alice:example.org",
			wantOK: true,
		},
		{
			name: "email with subdomain",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumEmail, ID: "bob@mail.example.co.uk"},
			},
			wantID: "@bolte/mail.example.co.uk/bob:example.org",
			wantOK: true,
		},
		{
			name: "email with plus tag",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumEmail, ID: "user+tag@domain.com"},
			},
			wantID: "@bolte/domain.com/user+tag:example.org",
			wantOK: true,
		},
		{
			name: "wrong medium",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumMatrix, ID: "alice@example.com"},
			},
			wantOK: false,
		},
		{
			name: "empty address ID",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumEmail, ID: ""},
			},
			wantOK: false,
		},
		{
			name: "missing at symbol",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumEmail, ID: "alice"},
			},
			wantOK: false,
		},
		{
			name: "empty localpart",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumEmail, ID: "@example.com"},
			},
			wantOK: false,
		},
		{
			name: "empty domain",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumEmail, ID: "alice@"},
			},
			wantOK: false,
		},
		{
			name: "multiple at symbols",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumEmail, ID: "alice@foo@bar.com"},
			},
			wantOK: false,
		},
		{
			name: "id longer than 255 characters uses hash fallback",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumEmail, ID: strings.Repeat("a", 200) + "@" + strings.Repeat("b", 100) + ".com"},
			},
			// SHA-256 hash of the ID
			wantID: "@bolte/0573bc1e0b1121eb1296cad34c7feff18ebd91880cd8047c9e0a5bc86b30b4ae:example.org",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := ghostSenderID(tt.sender, cfg)
			if gotOK != tt.wantOK {
				t.Fatalf("ghostSenderID() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotID != tt.wantID {
				t.Errorf("ghostSenderID() = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}

func TestAdapterSendSuccess(t *testing.T) {
	a := newTestAdapter()
	mock := a.client.(*mockClient)
	mock.sendEventID = "!sent-event:example.org"

	msg := relay.RoutedMessage{
		Message: relay.Message{
			Sender: relay.Identity{
				Address: relay.Address{
					Mode: relay.MediumEmail,
					ID:   "alice@example.com",
				},
				DisplayName: "Alice Wonderland",
			},
			InReplyTo: "$parent-event",
			Body:      "Hello from test!",
		},
		To: relay.Address{
			Mode: relay.MediumMatrix,
			ID:   a.cfg.RoomID,
		},
	}

	eventID, err := a.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if eventID != "!sent-event:example.org" {
		t.Errorf("Send() = %q, want %q", eventID, "!sent-event:example.org")
	}

	if len(mock.sentEvents) != 1 {
		t.Fatalf("len(sentEvents) = %d, want 1", len(mock.sentEvents))
	}
	sent := mock.sentEvents[0]
	if sent.RoomID != a.cfg.RoomID {
		t.Errorf("sent.RoomID = %q, want %q", sent.RoomID, a.cfg.RoomID)
	}
	wantSender := "@bolte/example.com/alice:example.org"
	if sent.Sender != wantSender {
		t.Errorf("sent.Sender = %q, want %q", sent.Sender, wantSender)
	}
	if sent.DisplayName != "Alice Wonderland" {
		t.Errorf("sent.DisplayName = %q, want %q", sent.DisplayName, "Alice Wonderland")
	}
	if sent.ReplyTo != "$parent-event" {
		t.Errorf("sent.ReplyTo = %q, want %q", sent.ReplyTo, "$parent-event")
	}
	if sent.Body != "Hello from test!" {
		t.Errorf("sent.Body = %q, want %q", sent.Body, "Hello from test!")
	}
}

func TestAdapterSendDropped(t *testing.T) {
	tests := []struct {
		name   string
		sender relay.Identity
	}{
		{
			name:   "empty sender ID",
			sender: relay.Identity{Address: relay.Address{Mode: relay.MediumEmail, ID: ""}},
		},
		{
			name: "wrong medium",
			sender: relay.Identity{
				Address: relay.Address{Mode: relay.MediumMatrix, ID: "alice@example.com"},
			},
		},
		{
			name:   "malformed email without domain",
			sender: relay.Identity{Address: relay.Address{Mode: relay.MediumEmail, ID: "alice@"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestAdapter()
			mock := a.client.(*mockClient)

			msg := relay.RoutedMessage{
				Message: relay.Message{
					Sender: tt.sender,
					Body:   "Dropped message",
				},
			}

			eventID, err := a.Send(context.Background(), msg)
			if err != nil {
				t.Fatalf("Send() error = %v, want nil", err)
			}
			if eventID != "" {
				t.Errorf("Send() = %q, want empty string", eventID)
			}
			if len(mock.sentEvents) != 0 {
				t.Errorf("client.Send was called %d times, want 0", len(mock.sentEvents))
			}
		})
	}
}

func TestAdapterSendClientError(t *testing.T) {
	a := newTestAdapter()
	mock := a.client.(*mockClient)
	mock.sendErr = errors.New("network failure")

	msg := relay.RoutedMessage{
		Message: relay.Message{
			Sender: relay.Identity{
				Address: relay.Address{
					Mode: relay.MediumEmail,
					ID:   "alice@example.com",
				},
			},
			Body: "Will fail",
		},
	}

	eventID, err := a.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("Send() error = nil, want error")
	}
	if eventID != "" {
		t.Errorf("Send() = %q, want empty string", eventID)
	}
	if !strings.HasPrefix(err.Error(), "matrix: failed to send:") {
		t.Errorf("Send() error = %q, want prefix 'matrix: failed to send:'", err.Error())
	}
}

// TestCommitEmptyCursor checks that Commit with an empty cursor commits the
// last seen EventID from the previous fetch.
func TestCommitEmptyCursor(t *testing.T) {
	ctx := context.Background()
	a := newTestAdapter()
	a.cfg.RoomID = "!room-commit-empty:example.org"
	a.lastEventID = "!latest-event:example.org"

	if err := a.Commit(ctx, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	gotEventID, err := a.getCursor(ctx)
	if err != nil {
		t.Fatalf("getCursor: %v", err)
	}
	if gotEventID != a.lastEventID {
		t.Errorf("persisted EventID = %q, want %q", gotEventID, a.lastEventID)
	}
}

// TestCommitEventIDCursor checks that Commit with a valid EventID cursor
// commits that specific EventID.
func TestCommitEventIDCursor(t *testing.T) {
	ctx := context.Background()
	a := newTestAdapter()
	a.cfg.RoomID = "!room-commit-eventid:example.org"

	wantEventID := "!event-1:example.org"
	a.lastEventID = "!event-2:example.org"
	a.lastFetched = map[string]bool{
		wantEventID:            true,
		"!event-2:example.org": true,
	}

	if err := a.Commit(ctx, wantEventID); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	gotEventID, err := a.getCursor(ctx)
	if err != nil {
		t.Fatalf("getCursor: %v", err)
	}
	if gotEventID != wantEventID {
		t.Errorf("persisted EventID = %q, want %q", gotEventID, wantEventID)
	}
}

// TestCommitCursorNotFound checks that Commit returns an error when the given
// cursor is not found in lastFetched.
func TestCommitCursorNotFound(t *testing.T) {
	ctx := context.Background()
	a := newTestAdapter()
	a.cfg.RoomID = "!room-commit-notfound:example.org"
	a.lastFetched = map[string]bool{
		"!event-1:example.org": true,
	}

	cursor := "!unknown-event:example.org"
	err := a.Commit(ctx, cursor)
	if err == nil {
		t.Fatal("Commit expected error for unknown cursor, got nil")
	}

	wantErr := fmt.Sprintf(
		"matrix: failed to commit: cursor %q not found in fetched events",
		cursor,
	)
	if err.Error() != wantErr {
		t.Errorf("Commit error = %q, want %q", err.Error(), wantErr)
	}
}

// TestCommitStoreError checks that Commit propagates errors from the store.
func TestCommitStoreError(t *testing.T) {
	a := newTestAdapter()
	a.cfg.RoomID = "!room-commit-store-error:example.org"
	a.lastEventID = "!event-1:example.org"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := a.Commit(ctx, "")
	if err == nil {
		t.Fatal("Commit expected error for canceled context, got nil")
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
