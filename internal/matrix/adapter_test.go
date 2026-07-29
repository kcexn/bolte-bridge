package matrix

import (
	"context"
	"testing"

	"bolte-bridge/internal/relay"
)

type mockClient struct {
	closeCalled bool
}

func (m *mockClient) Fetch(context.Context, string) ([]RawEvent, error) {
	return nil, nil
}

func (m *mockClient) Send(context.Context, OutboundEvent) error {
	return nil
}

func (m *mockClient) Close(context.Context) error {
	m.closeCalled = true
	return nil
}

func TestAdapterMedium(t *testing.T) {
	a := &Adapter{}

	if got := a.Medium(); got != relay.MediumMatrix {
		t.Fatalf("Medium() = %v, want %v", got, relay.MediumMatrix)
	}
}

func TestAdapterFetch(t *testing.T) {
	a := &Adapter{}

	msgs, err := a.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if msgs != nil {
		t.Fatalf("Fetch() = %v, want nil", msgs)
	}
}

func TestAdapterSend(t *testing.T) {
	a := &Adapter{}

	id, err := a.Send(context.Background(), relay.RoutedMessage{})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if id != "" {
		t.Fatalf("Send() = %q, want empty string", id)
	}
}

func TestAdapterCommit(t *testing.T) {
	a := &Adapter{}

	if err := a.Commit(context.Background(), ""); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
}

func TestAdapterClose(t *testing.T) {
	client := &mockClient{}
	a := &Adapter{client: client}

	if err := a.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !client.closeCalled {
		t.Fatal("client.Close was not called")
	}
}

func TestNewAdapterInvalidConfig(t *testing.T) {
	_, err := NewAdapter(context.Background(), Config{})
	if err == nil {
		t.Fatal("NewAdapter() error = nil, want validation error")
	}
}
