package email

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"

	"bolte-bridge/internal/relay"
	"bolte-bridge/internal/store"
)

func (a *Adapter) fetch(ctx context.Context) ([]relay.Message, error) {
	uidCursor, uidValidity, err := a.getCursor(ctx)
	uidNext, mboxUIDValidity := a.client.LatestUID()
	switch {
	// Either no cursor has been set yet, or the mailbox UIDValidity has changed.
	case errors.Is(err, sql.ErrNoRows) || uidValidity != mboxUIDValidity:
		return nil, a.setCursor(ctx, uidNext-1, mboxUIDValidity)
	case err != nil:
		return nil, err
	default:
		// Fetch all messages from the committed cursor+1.
		return a.fetchMessages(ctx, uidCursor+1)
	}
}

func (a *Adapter) fetchMessages(ctx context.Context, startUID uint32) ([]relay.Message, error) {
	rawMessages, err := a.client.Fetch(ctx, startUID)
	if err != nil {
		return nil, err
	}

	messages, err := rawMessagesToRelayMessages(rawMessages)
	if err != nil {
		return nil, err
	}

	msgIDToUID, err := makeIDToUIDMap(rawMessages, messages)
	if err != nil {
		return nil, err
	}
	a.msgIDToUID = msgIDToUID

	return messages, nil
}

// getCursor retrieves the current IMAP UID and UIDValidity from the store.
func (a *Adapter) getCursor(ctx context.Context) (uint32, uint32, error) {
	var uid, uidValidity uint32
	err := store.Client().WithTx(ctx, func(ctx context.Context, tx store.Tx) error {
		var err error
		uid, uidValidity, err = tx.Cursor(ctx, a.cfg.Username, a.cfg.Mailbox)
		return err
	})
	return uid, uidValidity, err
}

// setCursor durably persists the given UID and UIDValidity to the store.
func (a *Adapter) setCursor(ctx context.Context, uid, uidValidity uint32) error {
	return store.Client().WithTx(ctx, func(ctx context.Context, tx store.Tx) error {
		return tx.SetCursor(ctx, a.cfg.Username, a.cfg.Mailbox, uid, uidValidity)
	})
}

// rawMessagesToRelayMessages converts a slice of RawMessages to relay.Messages,
// returning both the messages and a msgIDToUID map for cursor tracking.
func rawMessagesToRelayMessages(
	rawMessages []RawMessage,
) ([]relay.Message, error) {
	messages := make([]relay.Message, len(rawMessages))

	for i, raw := range rawMessages {
		msg, err := rawMessageToRelayMessage(raw)
		if err != nil {
			return nil, err
		}
		messages[i] = msg

	}
	return messages, nil
}

// rawMessageToRelayMessage parses a raw RFC 822 message and converts it to a relay.Message.
func rawMessageToRelayMessage(raw RawMessage) (relay.Message, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw.Raw))
	if err != nil {
		return relay.Message{}, err
	}

	// Extract sender from From header.
	fromAddr, err := mail.ParseAddress(msg.Header.Get("From"))
	if err != nil {
		return relay.Message{}, err
	}

	sender := relay.Identity{
		Address: relay.Address{
			Mode: relay.MediumEmail,
			ID:   fromAddr.Address,
		},
		DisplayName: fromAddr.Name,
	}

	// Extract Message-ID.
	messageID := msg.Header.Get("Message-ID")

	// Extract thread ID from In-Reply-To header.
	threadID := msg.Header.Get("In-Reply-To")

	// Extract plain text body from the message.
	body, err := extractPlainTextBody(msg)
	if err != nil {
		return relay.Message{}, err
	}

	return relay.Message{
		Sender:    sender,
		MessageID: messageID,
		ThreadID:  threadID,
		Body:      body,
	}, nil
}

// extractPlainTextBody extracts the plain text body from an email message,
// stripping HTML and rich text formatting.
func extractPlainTextBody(msg *mail.Message) (string, error) {
	contentType := msg.Header.Get("Content-Type")
	if contentType == "" {
		// If no Content-Type, treat as plain text.
		body, err := io.ReadAll(msg.Body)
		return string(body), err
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// If we can't parse the content type, treat as plain text.
		body, err := io.ReadAll(msg.Body)
		return string(body), err
	}

	// If not multipart, just read and return the body.
	if !strings.HasPrefix(mediaType, "multipart/") {
		body, err := io.ReadAll(msg.Body)
		return string(body), err
	}

	// For multipart messages, find the plain text part.
	boundary := params["boundary"]
	if boundary == "" {
		body, err := io.ReadAll(msg.Body)
		return string(body), err
	}

	reader := multipart.NewReader(msg.Body, boundary)
	var plainTextBody string

	for len(plainTextBody) == 0 {
		part, err := reader.NextPart()
		if err != nil && err != io.EOF {
			return "", err
		}
		if err == io.EOF {
			break
		}

		partContentType := part.Header.Get("Content-Type")
		if partContentType == "" {
			// Default to text/plain if no Content-Type.
			partContentType = "text/plain"
		}

		partMediaType, _, err := mime.ParseMediaType(partContentType)
		if err != nil {
			continue
		}

		// Prefer text/plain, but fall back to any text/* part if needed.
		if partMediaType == "text/plain" {
			body, err := io.ReadAll(part)
			if err != nil {
				return "", err
			}
			plainTextBody = string(body)
		}
		if plainTextBody == "" && strings.HasPrefix(partMediaType, "text/") {
			body, err := io.ReadAll(part)
			if err != nil {
				return "", err
			}
			plainTextBody = string(body)
		}
	}

	return plainTextBody, nil
}

func makeIDToUIDMap(
	rawMessages []RawMessage,
	relayMessages []relay.Message,
) (map[string]uint32, error) {
	if len(rawMessages) != len(relayMessages) {
		return nil, fmt.Errorf("email: makeIDToUIDMap: length of raw and relay messages differ")
	}
	msgIDToUID := make(map[string]uint32, len(rawMessages))
	for i, msg := range relayMessages {
		raw := rawMessages[i]
		msgIDToUID[msg.MessageID] = raw.UID
	}
	return msgIDToUID, nil
}
