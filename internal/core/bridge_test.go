package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bolte-bridge/internal/relay"
)

// mockAdapter is a scriptable core.Adapter for Bridge tests. It records every
// call (across both adapters, via a shared log) and can be told to fail a given
// method with a sentinel error. fetched is returned verbatim from Fetch.
type mockAdapter struct {
	medium  relay.Medium
	name    string // prefix used in the shared call log ("matrix"/"email")
	log     *[]string
	fetched []relay.Message

	fetchErr  error
	sendErr   error
	commitErr error

	sent []relay.RoutedMessage
}

func (m *mockAdapter) Medium() relay.Medium { return m.medium }

func (m *mockAdapter) Fetch(context.Context) ([]relay.Message, error) {
	*m.log = append(*m.log, m.name+".Fetch")
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.fetched, nil
}

func (m *mockAdapter) Send(_ context.Context, msg relay.RoutedMessage) error {
	*m.log = append(*m.log, m.name+".Send")
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockAdapter) Commit(context.Context) error {
	*m.log = append(*m.log, m.name+".Commit")
	return m.commitErr
}

// newAdapters wires an email and a matrix mock adapter to share one call log,
// so tests can assert the exact interleaving of calls across both directions.
func newAdapters() (email, matrix *mockAdapter, log *[]string) {
	shared := &[]string{}
	email = &mockAdapter{medium: relay.MediumEmail, name: "email", log: shared}
	matrix = &mockAdapter{medium: relay.MediumMatrix, name: "matrix", log: shared}
	return email, matrix, shared
}

// Compile-time assertion that the mock satisfies the interface under test.
var _ Adapter = (*mockAdapter)(nil)

// Run: the happy path. room→list fetches Matrix, publishes to email, and
// commits Matrix; list→room then fetches email, publishes to Matrix, and
// commits email. The ordering is asserted across both adapters.
func TestRunSuccess(t *testing.T) {
	email, matrix, log := newAdapters()
	matrix.fetched = []relay.Message{{Body: "from-room"}}
	email.fetched = []relay.Message{{Body: "from-list"}}

	b := NewBridge(email, matrix)
	if err := b.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"matrix.Fetch", "email.Send", "matrix.Commit",
		"email.Fetch", "matrix.Send", "email.Commit",
	}
	if got := *log; !equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
	if len(email.sent) != 1 || email.sent[0].Message.Body != "from-room" {
		t.Fatalf("email.sent = %+v, want the room message", email.sent)
	}
	if len(matrix.sent) != 1 || matrix.sent[0].Message.Body != "from-list" {
		t.Fatalf("matrix.sent = %+v, want the list message", matrix.sent)
	}
}

// Run: a Fetch error aborts the tick before any Send or Commit, and the second
// direction never runs.
func TestRunFetchError(t *testing.T) {
	email, matrix, log := newAdapters()
	matrix.fetchErr = errBoom

	b := NewBridge(email, matrix)
	if err := b.Run(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
	if got := *log; !equal(got, []string{"matrix.Fetch"}) {
		t.Fatalf("call order = %v, want only the failed fetch", got)
	}
}

// Run: a Send error (surfaced through the engine's Publish) aborts before the
// source cursor is committed, so the batch replays next run.
func TestRunSendError(t *testing.T) {
	email, matrix, log := newAdapters()
	matrix.fetched = []relay.Message{{Body: "from-room"}}
	email.sendErr = errBoom

	b := NewBridge(email, matrix)
	if err := b.Run(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
	if got := *log; !equal(got, []string{"matrix.Fetch", "email.Send"}) {
		t.Fatalf("call order = %v, want fetch+send with no commit", got)
	}
}

// Run: a Commit error propagates, wrapped with direction context.
func TestRunCommitError(t *testing.T) {
	email, matrix, _ := newAdapters()
	matrix.commitErr = errBoom

	b := NewBridge(email, matrix)
	err := b.Run(context.Background())
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
	if got := err.Error(); !strings.Contains(got, "relay room->list") {
		t.Fatalf("err = %q, want it wrapped with the direction", got)
	}
}

// Run: a failure in the second direction (list->room) is wrapped with that
// direction's context, and only after the first direction has fully completed.
func TestRunSecondDirectionError(t *testing.T) {
	email, matrix, log := newAdapters()
	matrix.fetched = []relay.Message{{Body: "from-room"}}
	email.fetchErr = errBoom

	b := NewBridge(email, matrix)
	err := b.Run(context.Background())
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
	if got := err.Error(); !strings.Contains(got, "relay list->room") {
		t.Fatalf("err = %q, want it wrapped with the list->room direction", got)
	}
	want := []string{
		"matrix.Fetch", "email.Send", "matrix.Commit", // first direction completes
		"email.Fetch", // second direction fails at its fetch
	}
	if got := *log; !equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

// Run: a failure in the first direction short-circuits the second — the email
// adapter is never touched.
func TestRunFirstDirectionShortCircuits(t *testing.T) {
	email, matrix, log := newAdapters()
	matrix.fetchErr = errBoom

	b := NewBridge(email, matrix)
	_ = b.Run(context.Background())

	for _, call := range *log {
		if strings.HasPrefix(call, "email") {
			t.Fatalf("email adapter was touched after first-direction failure: %v", *log)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
