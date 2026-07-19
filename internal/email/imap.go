package email

import (
	"context"
	"fmt"
	"sort"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// searchSince returns the UIDs strictly greater than sinceUID.
func searchSince(client *imapclient.Client, sinceUID uint32) ([]imap.UID, error) {
	var set imap.UIDSet
	set.AddRange(imap.UID(sinceUID+1), 0) // stop 0 means "*", the highest UID

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

	sort.Slice(msgs, func(i, j int) bool { return msgs[i].UID < msgs[j].UID })
	return msgs, nil
}

func (c *emailClient) Fetch(ctx context.Context, sinceUID uint32) ([]RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client, err := imapclient.DialTLS(c.cfg.IMAPAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("email: dial IMAP %s: %w", c.cfg.IMAPAddr, err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Login(c.cfg.Username, c.cfg.Password).Wait(); err != nil {
		return nil, fmt.Errorf("email: IMAP login: %w", err)
	}

	mbox, err := client.Select(c.cfg.Mailbox, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("email: select mailbox %q: %w", c.cfg.Mailbox, err)
	}

	uids, err := searchSince(client, sinceUID)
	if err != nil {
		return nil, err
	}
	if len(uids) == 0 {
		return nil, nil
	}

	msgs, err := fetchUIDs(client, uids, mbox.UIDValidity)
	if err != nil {
		return nil, err
	}

	if err := client.Logout().Wait(); err != nil {
		return nil, fmt.Errorf("email: IMAP logout: %w", err)
	}
	return msgs, nil
}
