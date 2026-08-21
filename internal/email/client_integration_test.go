//go:build integration

package email

import (
	"context"
	"os"
	"testing"
)

func TestDialAndSelectMailbox_RealServer(t *testing.T) {
	cfg := Config{
		IMAPAddr: os.Getenv("CI_IMAP_ADDR"),
		Username: os.Getenv("CI_IMAP_USER"),
		Password: os.Getenv("CI_IMAP_PASS"),
		Mailbox:  "INBOX",
	}

	info, err := dialAndSelectMailbox(cfg)
	if err != nil {
		t.Fatalf("failed to connect to real IMAP server: %v", err)
	}
	defer func() { _ = info.Client.Close() }()

	if info.SelectData == nil {
		t.Fatal("expected mailbox info, got nil")
	}
}

func TestNewClient_RealServer(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		IMAPAddr:      os.Getenv("CI_IMAP_ADDR"),
		Username:      os.Getenv("CI_IMAP_USER"),
		Password:      os.Getenv("CI_IMAP_PASS"),
		Mailbox:       os.Getenv("CI_IMAP_MBOX"),
		SMTPAddr:      os.Getenv("CI_SMTP_ADDR"),
		MessageDomain: "example.com",
	}

	client, err := NewClient(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to construct Client: %v", err)
	}
	defer func() { _ = client.Close(ctx) }()

	if client == nil {
		t.Fatal("expected non-nil Client")
	}
}

func TestNewEmailClient_RealServer(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		IMAPAddr:      os.Getenv("CI_IMAP_ADDR"),
		Username:      os.Getenv("CI_IMAP_USER"),
		Password:      os.Getenv("CI_IMAP_PASS"),
		Mailbox:       os.Getenv("CI_IMAP_MBOX"),
		SMTPAddr:      os.Getenv("CI_SMTP_ADDR"),
		MessageDomain: "example.com",
	}

	client, err := newEmailClient(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create email client: %v", err)
	}
	defer func() { _ = client.Close(ctx) }()

	if client.mbox == nil || client.mbox.Client == nil {
		t.Fatal("expected non-nil mailbox client")
	}
	if client.mbox.SelectData == nil {
		t.Fatal("expected non-nil mailbox SelectData")
	}
}

func TestLatestUID_RealServer(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		IMAPAddr:      os.Getenv("CI_IMAP_ADDR"),
		Username:      os.Getenv("CI_IMAP_USER"),
		Password:      os.Getenv("CI_IMAP_PASS"),
		Mailbox:       os.Getenv("CI_IMAP_MBOX"),
		SMTPAddr:      os.Getenv("CI_SMTP_ADDR"),
		MessageDomain: "example.com",
	}

	client, err := newEmailClient(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create email client: %v", err)
	}
	defer func() { _ = client.Close(ctx) }()

	uidNext, uidValidity := client.LatestUID()
	if uidNext == 0 {
		t.Error("expected non-zero UIDNext from real IMAP server")
	}
	if uidValidity == 0 {
		t.Error("expected non-zero UIDValidity from real IMAP server")
	}
	if uidNext != uint32(client.mbox.SelectData.UIDNext) {
		t.Errorf("LatestUID() UIDNext = %d; want %d", uidNext, client.mbox.SelectData.UIDNext)
	}
	if uidValidity != client.mbox.SelectData.UIDValidity {
		t.Errorf(
			"LatestUID() UIDValidity = %d; want %d",
			uidValidity,
			client.mbox.SelectData.UIDValidity,
		)
	}
}

func TestClose_RealServer(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		IMAPAddr:      os.Getenv("CI_IMAP_ADDR"),
		Username:      os.Getenv("CI_IMAP_USER"),
		Password:      os.Getenv("CI_IMAP_PASS"),
		Mailbox:       os.Getenv("CI_IMAP_MBOX"),
		SMTPAddr:      os.Getenv("CI_SMTP_ADDR"),
		MessageDomain: "example.com",
	}

	client, err := newEmailClient(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create email client: %v", err)
	}

	if err := client.Close(ctx); err != nil {
		t.Fatalf("failed to close email client: %v", err)
	}
}
