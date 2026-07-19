package email

import (
	"bytes"
	"context"
	"fmt"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

func (c *emailClient) Send(ctx context.Context, from string, to []string, raw []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	client, err := smtp.DialStartTLS(c.cfg.SMTPAddr, nil)
	if err != nil {
		return fmt.Errorf("email: dial SMTP %s: %w", c.cfg.SMTPAddr, err)
	}
	defer func() { _ = client.Close() }()

	auth := sasl.NewPlainClient("", c.cfg.Username, c.cfg.Password)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("email: SMTP auth: %w", err)
	}

	if err := client.SendMail(from, to, bytes.NewReader(raw)); err != nil {
		return fmt.Errorf("email: SMTP send: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("email: SMTP quit: %w", err)
	}
	return nil
}

// Compile-time assertion that emailClient satisfies Client.
var _ Client = (*emailClient)(nil)
