package email

import (
	"testing"
)

// validConfig returns a Config with every required field populated, so each
// test case can zero out exactly the one field it exercises.
func validConfig() Config {
	return Config{
		Username:      "user@example.com",
		Password:      "hunter2",
		IMAPAddr:      "imap.example.com:993",
		SMTPAddr:      "smtp.example.com:587",
		MessageDomain: "example.com",
		Mailbox:       "INBOX",
	}
}

func TestValidateConfig(t *testing.T) {
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
		{
			name:    "missing message domain",
			mutate:  func(c *Config) { c.MessageDomain = "" },
			wantErr: "email: Config.MessageDomain is required",
		},
		{
			name:    "all fields valid",
			mutate:  func(c *Config) {},
			wantErr: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)

			err := validateConfig(cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("validateConfig(%+v) = %v; want nil", cfg, err)
				}
				return
			}
			if err == nil {
				t.Errorf("validateConfig(%+v) = nil; want error %q", cfg, tc.wantErr)
			} else if got := err.Error(); got != tc.wantErr {
				t.Errorf("validateConfig error = %q, want %q", got, tc.wantErr)
			}
		})
	}
}
