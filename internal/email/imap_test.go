package email

import (
	"testing"
)

func TestSortRawMessages(t *testing.T) {
	msgs := []RawMessage{
		{UID: 42},
		{UID: 12},
		{UID: 99},
		{UID: 5},
	}

	sortRawMessages(msgs)

	if len(msgs) != 4 {
		t.Fatalf("got length %d, want 4", len(msgs))
	}

	expected := []uint32{5, 12, 42, 99}
	for i, u := range expected {
		if msgs[i].UID != u {
			t.Errorf("at index %d: got UID %d, want %d", i, msgs[i].UID, u)
		}
	}
}
