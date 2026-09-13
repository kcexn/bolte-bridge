package email

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// searchSince returns the UIDs greater than or equal to startUID.
func searchSince(client *imapclient.Client, startUID uint32) ([]imap.UID, error) {
	var set imap.UIDSet
	set.AddRange(imap.UID(startUID), 0) // stop 0 means "*", the highest UID

	criteria := &imap.SearchCriteria{UID: []imap.UIDSet{set}}
	data, err := client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("email: UID search: %w", err)
	}
	return data.AllUIDs(), nil
}

// fetchUIDs fetches the full body and internal date for each UID and returns
// them oldest (lowest UID) first, stamped with the mailbox's UIDVALIDITY.
func fetchUIDs(
	client *imapclient.Client,
	uids []imap.UID,
	uidValidity uint32,
) ([]RawMessage, error) {
	opts := &imap.FetchOptions{
		UID:          true,
		InternalDate: true,
		// An empty body section fetches the entire message (BODY[]).
		BodySection: []*imap.FetchItemBodySection{{}},
	}

	buffers, err := client.Fetch(imap.UIDSetNum(uids...), opts).Collect()
	if err != nil {
		return nil, fmt.Errorf("email: UID fetch: %w", err)
	}

	msgs := make([]RawMessage, 0, len(buffers))
	for _, buf := range buffers {
		var raw []byte
		if len(buf.BodySection) > 0 {
			raw = buf.BodySection[0].Bytes
		}
		msgs = append(msgs, RawMessage{
			UID:          uint32(buf.UID),
			UIDValidity:  uidValidity,
			InternalDate: buf.InternalDate,
			Raw:          raw,
		})
	}

	slices.SortFunc(msgs, func(a, b RawMessage) int {
		return cmp.Compare(a.UID, b.UID)
	})
	return msgs, nil
}

func (c *emailClient) Fetch(ctx context.Context, startUID uint32) ([]RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client := c.mbox.Client
	uidValidity := c.mbox.SelectData.UIDValidity

	uids, err := searchSince(client, startUID)
	if err != nil {
		return nil, err
	}
	if len(uids) == 0 {
		return nil, nil
	}

	msgs, err := fetchUIDs(client, uids, uidValidity)
	if err != nil {
		return nil, err
	}

	return msgs, nil
}
