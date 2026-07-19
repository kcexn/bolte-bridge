package config

import (
	"errors"
	"strings"
	"testing"

	"bolte-bridge/internal/email"
)

func TestLoadStoreDBPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{
			name: "default when neither flag nor env is set",
			want: defaultDBPath,
		},
		{
			name: "environment variable overrides the default",
			env:  map[string]string{"BOLTE_BRIDGE_DB_PATH": "/var/lib/bridge/env.db"},
			want: "/var/lib/bridge/env.db",
		},
		{
			name: "long flag overrides both the environment and the default",
			args: []string{"--db-path", "/tmp/flag.db"},
			env:  map[string]string{"BOLTE_BRIDGE_DB_PATH": "/var/lib/bridge/env.db"},
			want: "/tmp/flag.db",
		},
		{
			name: "short flag overrides both the environment and the default",
			args: []string{"-d", "/tmp/flag.db"},
			env:  map[string]string{"BOLTE_BRIDGE_DB_PATH": "/var/lib/bridge/env.db"},
			want: "/tmp/flag.db",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			// Load only the section under test: other DefaultSections have their
			// own required settings, so exercising them here would couple this
			// store-path test to unrelated configuration as the app grows.
			cfg, err := Load(tc.args, storeSection)
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			if got := cfg.Store.SQLite.Path; got != tc.want {
				t.Errorf("Store.SQLite.Path = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoadUnknownFlagErrors(t *testing.T) {
	_, err := Load([]string{"--nope"}, DefaultSections...)
	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("Load returned %v, want ErrInvalidArguments", err)
	}
}

func TestLoadHelpReturnsErrHelp(t *testing.T) {
	_, err := Load([]string{"--help"}, DefaultSections...)
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("Load returned %v, want ErrHelp", err)
	}
}

func TestLoadDBPathValidationErrorAborts(t *testing.T) {

	// An empty db.path is rejected by storeSection's ApplyFunc.
	_, err := Load([]string{"--db-path", ""}, storeSection)
	if err == nil {
		t.Fatal("Load with empty db.path returned nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), "db.path") {
		t.Errorf("error %q does not mention the offending key db.path", err)
	}
}

func TestLoadEmailConfig(t *testing.T) {
	// account and password are required, so every success case supplies them; the
	// endpoint fields exercise default/env/flag precedence.
	const account = "BOLTE_BRIDGE_EMAIL_ACCOUNT"
	const password = "BOLTE_BRIDGE_EMAIL_PASSWORD"

	tests := []struct {
		name string
		args []string
		env  map[string]string
		want email.Config
	}{
		{
			name: "endpoint defaults when only credentials are set",
			env:  map[string]string{account: "bridge@example.org", password: "app-password"},
			want: email.Config{
				Username: "bridge@example.org",
				Password: "app-password",
				IMAPAddr: defaultIMAPAddr,
				SMTPAddr: defaultSMTPAddr,
				Mailbox:  defaultMailbox,
			},
		},
		{
			name: "environment variables override endpoint defaults",
			env: map[string]string{
				account:                        "bridge@example.org",
				password:                       "app-password",
				"BOLTE_BRIDGE_EMAIL_IMAP_ADDR": "imap.example.org:1993",
				"BOLTE_BRIDGE_EMAIL_SMTP_ADDR": "smtp.example.org:1587",
				"BOLTE_BRIDGE_EMAIL_MAILBOX":   "Lists/lug",
			},
			want: email.Config{
				Username: "bridge@example.org",
				Password: "app-password",
				IMAPAddr: "imap.example.org:1993",
				SMTPAddr: "smtp.example.org:1587",
				Mailbox:  "Lists/lug",
			},
		},
		{
			name: "flags override both the environment and the defaults",
			args: []string{
				"--email", "flag@example.org",
				"--email-imap-addr", "imap.flag.org:993",
				"--email-smtp-addr", "smtp.flag.org:587",
				"--email-mailbox", "Flagged",
			},
			env: map[string]string{
				account:                        "env@example.org",
				password:                       "app-password",
				"BOLTE_BRIDGE_EMAIL_IMAP_ADDR": "imap.env.org:993",
				"BOLTE_BRIDGE_EMAIL_SMTP_ADDR": "smtp.env.org:587",
				"BOLTE_BRIDGE_EMAIL_MAILBOX":   "Enved",
			},
			want: email.Config{
				Username: "flag@example.org",
				Password: "app-password",
				IMAPAddr: "imap.flag.org:993",
				SMTPAddr: "smtp.flag.org:587",
				Mailbox:  "Flagged",
			},
		},
		{
			name: "short account flag overrides the environment",
			args: []string{"-e", "short@example.org"},
			env:  map[string]string{account: "env@example.org", password: "app-password"},
			want: email.Config{
				Username: "short@example.org",
				Password: "app-password",
				IMAPAddr: defaultIMAPAddr,
				SMTPAddr: defaultSMTPAddr,
				Mailbox:  defaultMailbox,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg, err := Load(tc.args, emailSection)
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			if cfg.Email != tc.want {
				t.Errorf("cfg.Email = %+v, want %+v", cfg.Email, tc.want)
			}
		})
	}
}

func TestLoadEmailAccountValidationErrorAborts(t *testing.T) {
	// An empty email.account is rejected by emailSection's ApplyFunc, even with a
	// password present.
	t.Setenv("BOLTE_BRIDGE_EMAIL_PASSWORD", "app-password")
	_, err := Load([]string{"--email", ""}, emailSection)
	if err == nil {
		t.Fatal("Load with empty email.account returned nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), "email.account") {
		t.Errorf("error %q does not mention the offending key email.account", err)
	}
}

func TestLoadEmailPasswordValidationErrorAborts(t *testing.T) {
	// The password is env-only; an account with no password is rejected by
	// emailSection's ApplyFunc. Clearing the variable guards against ambient
	// values leaking in from the test host.
	t.Setenv("BOLTE_BRIDGE_EMAIL_PASSWORD", "")
	_, err := Load([]string{"--email", "bridge@example.org"}, emailSection)
	if err == nil {
		t.Fatal("Load with empty email.password returned nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), "email.password") {
		t.Errorf("error %q does not mention the offending key email.password", err)
	}
}

func TestLoadBindingErrorAborts(t *testing.T) {
	failing := func(_ *Binder) (ApplyFunc, error) {
		return nil, errors.New("boom")
	}
	if _, err := Load(nil, failing); err == nil {
		t.Fatal("Load with a failing section returned nil error, want non-nil")
	}
}
