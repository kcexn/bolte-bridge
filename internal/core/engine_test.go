package core

import (
	"context"
	"errors"
	"testing"

	"bolte-bridge/internal/relay"
)

// errBoom is a sentinel used to assert that a stage's error is propagated
// verbatim rather than wrapped or swallowed.
var errBoom = errors.New("boom")

// --- Process ---

// Process: the nil-Publish guard. Process must panic rather than run the
// pipeline with no terminal sink.
func TestProcessNilPublishPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Process did not panic with nil Publish")
		}
	}()

	e := &RelayEngine{}
	_ = e.Process(context.Background(), []relay.Message{{}})
}

// Process: the loop-completion branch. Every stage succeeds and each kept
// message reaches Publish. The empty-slice case also lands here, returning
// nil without publishing.
func TestProcessSuccess(t *testing.T) {
	var published []relay.RoutedMessage
	e := &RelayEngine{
		Filters: []FilterFunc{
			func(context.Context, relay.Message) (bool, error) { return true, nil },
		},
		Sanitizers: []SanitizeFunc{
			func(_ context.Context, m relay.Message) (relay.Message, error) { m.Body += "!"; return m, nil },
		},
		Mappers: []MapFunc{
			func(_ context.Context, rm relay.RoutedMessage) (relay.RoutedMessage, error) {
				rm.To.ID = "dst"
				return rm, nil
			},
		},
		Publish: func(_ context.Context, rm relay.RoutedMessage) error {
			published = append(published, rm)
			return nil
		},
	}

	msgs := []relay.Message{{Body: "a"}, {Body: "b"}}
	if err := e.Process(context.Background(), msgs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(published) != 2 {
		t.Fatalf("published %d messages, want 2", len(published))
	}
	if published[0].Message.Body != "a!" || published[0].To.ID != "dst" {
		t.Fatalf("published[0] = %+v, want sanitized+mapped message", published[0])
	}

	if err := e.Process(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error on empty slice: %v", err)
	}
}

// Process: the filter-error branch. A filter error aborts the whole tick and
// nothing is published.
func TestProcessFilterError(t *testing.T) {
	e := &RelayEngine{
		Filters: []FilterFunc{
			func(context.Context, relay.Message) (bool, error) { return false, errBoom },
		},
		Publish: func(context.Context, relay.RoutedMessage) error {
			t.Fatal("Publish ran after filter error")
			return nil
		},
	}

	if err := e.Process(context.Background(), []relay.Message{{}}); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

// Process: the !keep continue branch. A dropped message skips the rest of the
// pipeline, and processing continues with the next message.
func TestProcessFilterDrops(t *testing.T) {
	sanitized := false
	var published []relay.RoutedMessage
	e := &RelayEngine{
		Filters: []FilterFunc{
			func(_ context.Context, m relay.Message) (bool, error) { return m.Body == "keep", nil },
		},
		Sanitizers: []SanitizeFunc{
			func(_ context.Context, m relay.Message) (relay.Message, error) { sanitized = true; return m, nil },
		},
		Publish: func(_ context.Context, rm relay.RoutedMessage) error {
			published = append(published, rm)
			return nil
		},
	}

	msgs := []relay.Message{{Body: "drop"}, {Body: "keep"}}
	if err := e.Process(context.Background(), msgs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(published) != 1 || published[0].Message.Body != "keep" {
		t.Fatalf("published = %+v, want only the kept message", published)
	}
	if !sanitized {
		t.Fatal("kept message was not sanitized; loop did not continue past the drop")
	}
}

// Process: the sanitize-error branch. A sanitizer error aborts the tick and
// nothing is published.
func TestProcessSanitizeError(t *testing.T) {
	e := &RelayEngine{
		Filters: []FilterFunc{
			func(context.Context, relay.Message) (bool, error) { return true, nil },
		},
		Sanitizers: []SanitizeFunc{
			func(context.Context, relay.Message) (relay.Message, error) { return relay.Message{}, errBoom },
		},
		Publish: func(context.Context, relay.RoutedMessage) error {
			t.Fatal("Publish ran after sanitize error")
			return nil
		},
	}

	if err := e.Process(context.Background(), []relay.Message{{}}); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

// Process: the mapMessage-error branch. A mapper error aborts the tick and
// nothing is published.
func TestProcessMapError(t *testing.T) {
	e := &RelayEngine{
		Filters: []FilterFunc{
			func(context.Context, relay.Message) (bool, error) { return true, nil },
		},
		Mappers: []MapFunc{func(context.Context, relay.RoutedMessage) (relay.RoutedMessage, error) {
			return relay.RoutedMessage{}, errBoom
		}},
		Publish: func(context.Context, relay.RoutedMessage) error { t.Fatal("Publish ran after map error"); return nil },
	}

	if err := e.Process(context.Background(), []relay.Message{{}}); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

// Process: the Publish-error branch. A Publish failure is propagated and
// aborts the tick.
func TestProcessPublishError(t *testing.T) {
	e := &RelayEngine{
		Publish: func(context.Context, relay.RoutedMessage) error { return errBoom },
	}

	if err := e.Process(context.Background(), []relay.Message{{}}); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
}

// --- filter ---

// filter: the loop-completion branch. With every filter returning keep=true
// (and the degenerate empty-slice case), the message is kept.
func TestFilterAllKeep(t *testing.T) {
	e := &RelayEngine{
		Filters: []FilterFunc{
			func(context.Context, relay.Message) (bool, error) { return true, nil },
			func(context.Context, relay.Message) (bool, error) { return true, nil },
		},
	}

	keep, err := e.filter(context.Background(), relay.Message{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !keep {
		t.Fatal("keep = false, want true when every filter keeps")
	}
}

// filter: the err != nil branch. An erroring filter aborts, returning
// keep=false and the error, and short-circuits later filters.
func TestFilterError(t *testing.T) {
	called := false
	e := &RelayEngine{
		Filters: []FilterFunc{
			func(context.Context, relay.Message) (bool, error) { return false, errBoom },
			func(context.Context, relay.Message) (bool, error) { called = true; return true, nil },
		},
	}

	keep, err := e.filter(context.Background(), relay.Message{})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
	if keep {
		t.Fatal("keep = true, want false on error")
	}
	if called {
		t.Fatal("later filter ran after an error short-circuit")
	}
}

// filter: the !keep branch. The first rejecting filter drops the message
// (no error) and short-circuits later filters.
func TestFilterReject(t *testing.T) {
	called := false
	e := &RelayEngine{
		Filters: []FilterFunc{
			func(context.Context, relay.Message) (bool, error) { return false, nil },
			func(context.Context, relay.Message) (bool, error) { called = true; return true, nil },
		},
	}

	keep, err := e.filter(context.Background(), relay.Message{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keep {
		t.Fatal("keep = true, want false when a filter rejects")
	}
	if called {
		t.Fatal("later filter ran after a rejection short-circuit")
	}
}

// --- sanitize ---

// sanitize: the loop-completion branch. Sanitizers are chained, each
// receiving the previous one's output, and the final message is returned.
func TestSanitizeChains(t *testing.T) {
	e := &RelayEngine{
		Sanitizers: []SanitizeFunc{
			func(_ context.Context, m relay.Message) (relay.Message, error) {
				m.Body += "a"
				return m, nil
			},
			func(_ context.Context, m relay.Message) (relay.Message, error) {
				m.Body += "b"
				return m, nil
			},
		},
	}

	got, err := e.sanitize(context.Background(), relay.Message{Body: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Body != "xab" {
		t.Fatalf("Body = %q, want %q (sanitizers not chained in order)", got.Body, "xab")
	}
}

// sanitize: the err != nil branch. An erroring sanitizer aborts with a
// zero-valued Message and short-circuits later sanitizers.
func TestSanitizeError(t *testing.T) {
	called := false
	e := &RelayEngine{
		Sanitizers: []SanitizeFunc{
			func(_ context.Context, m relay.Message) (relay.Message, error) {
				return relay.Message{}, errBoom
			},
			func(_ context.Context, m relay.Message) (relay.Message, error) {
				called = true
				return m, nil
			},
		},
	}

	got, err := e.sanitize(context.Background(), relay.Message{Body: "x"})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
	if got != (relay.Message{}) {
		t.Fatalf("got = %+v, want zero Message on error", got)
	}
	if called {
		t.Fatal("later sanitizer ran after an error short-circuit")
	}
}

// --- mapMessage ---

// mapMessage: the loop-completion branch. The seed RoutedMessage wraps the
// sanitized Message, and mappers are folded through in order.
func TestMapMessageChains(t *testing.T) {
	e := &RelayEngine{
		Mappers: []MapFunc{
			func(_ context.Context, rm relay.RoutedMessage) (relay.RoutedMessage, error) {
				rm.To.ID += "1"
				return rm, nil
			},
			func(_ context.Context, rm relay.RoutedMessage) (relay.RoutedMessage, error) {
				rm.To.ID += "2"
				return rm, nil
			},
		},
	}

	msg := relay.Message{Body: "hello"}
	got, err := e.mapMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Message != msg {
		t.Fatalf("Message = %+v, want the seeded message %+v", got.Message, msg)
	}
	if got.To.ID != "12" {
		t.Fatalf("To.ID = %q, want %q (mappers not folded in order)", got.To.ID, "12")
	}
}

// mapMessage: the err != nil branch. An erroring mapper aborts with a
// zero-valued RoutedMessage and short-circuits later mappers.
func TestMapMessageError(t *testing.T) {
	called := false
	e := &RelayEngine{
		Mappers: []MapFunc{
			func(_ context.Context, rm relay.RoutedMessage) (relay.RoutedMessage, error) {
				return relay.RoutedMessage{}, errBoom
			},
			func(_ context.Context, rm relay.RoutedMessage) (relay.RoutedMessage, error) {
				called = true
				return rm, nil
			},
		},
	}

	got, err := e.mapMessage(context.Background(), relay.Message{Body: "hello"})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want errBoom", err)
	}
	if got != (relay.RoutedMessage{}) {
		t.Fatalf("got = %+v, want zero RoutedMessage on error", got)
	}
	if called {
		t.Fatal("later mapper ran after an error short-circuit")
	}
}
