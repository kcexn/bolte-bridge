package email

import (
	"testing"
)

// validConfig returns a Config with every required field populated, so each
// test case can zero out exactly the one field it exercises.
func validConfig() Config {
	return Config{
		Username: "user@example.com",
		Password: "hunter2",
		IMAPAddr: "imap.example.com:993",
		SMTPAddr: "smtp.example.com:587",
		Mailbox:  "INBOX",
	}
}

func TestNewEmailClientValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name:    "missing username",
			mutate:  func(c *Config) { c.Username = "" },
			wantErr: "email: Config.Username is required",
		},
		{
			name:    "missing password",
			mutate:  func(c *Config) { c.Password = "" },
			wantErr: "email: Config.Password is required",
		},
		{
			name:    "missing IMAP address",
			mutate:  func(c *Config) { c.IMAPAddr = "" },
			wantErr: "email: Config.IMAPAddr is required",
		},
		{
			name:    "missing SMTP address",
			mutate:  func(c *Config) { c.SMTPAddr = "" },
			wantErr: "email: Config.SMTPAddr is required",
		},
		{
			name:    "missing mailbox",
			mutate:  func(c *Config) { c.Mailbox = "" },
			wantErr: "email: Config.Mailbox is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)

			client, err := newEmailClient(cfg)
			if err == nil {
				t.Fatalf("newEmailClient(%+v) = %v, nil; want error %q", cfg, client, tc.wantErr)
			}
			if got := err.Error(); got != tc.wantErr {
				t.Errorf("newEmailClient error = %q, want %q", got, tc.wantErr)
			}
			if client != nil {
				t.Errorf("newEmailClient returned non-nil client %v alongside error", client)
			}
		})
	}
}

func TestNewEmailClientValid(t *testing.T) {
	cfg := validConfig()

	client, err := newEmailClient(cfg)
	if err != nil {
		t.Fatalf("newEmailClient(%+v) returned error: %v", cfg, err)
	}
	if client == nil {
		t.Fatal("newEmailClient returned nil client without error")
	}
	if client.cfg != cfg {
		t.Errorf("client.cfg = %+v, want %+v", client.cfg, cfg)
	}
}
