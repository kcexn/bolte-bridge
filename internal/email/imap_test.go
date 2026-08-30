package email

import (
	"testing"
)

func TestSortRawMessages(t *testing.T) {
	msgs := []RawMessage{
		{UID: 30},
		{UID: 10},
		{UID: 20},
	}

	sortRawMessages(msgs)

	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3", len(msgs))
	}
	if msgs[0].UID != 10 || msgs[1].UID != 20 || msgs[2].UID != 30 {
		t.Errorf("messages not sorted correctly: got UIDs [%d, %d, %d], want [10, 20, 30]",
			msgs[0].UID, msgs[1].UID, msgs[2].UID)
	}
}
