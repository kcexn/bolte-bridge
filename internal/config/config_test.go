package config

import (
	"errors"
	"strings"
	"testing"

	"bolte-bridge/internal/email"
	"bolte-bridge/internal/matrix"
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

func TestLoadMatrixConfig(t *testing.T) {
	// The matrix section applies no defaults, so every field is required. The env
	// case supplies them all; the flag case then overrides the non-secret fields.
	base := map[string]string{
		"BOLTE_BRIDGE_MATRIX_HOMESERVER_URL":   "https://matrix.example.org",
		"BOLTE_BRIDGE_MATRIX_SERVER_NAME":      "matrix.example.org",
		"BOLTE_BRIDGE_MATRIX_APPSERVICE_ID":    "bolte-bridge",
		"BOLTE_BRIDGE_MATRIX_AS_TOKEN":         "as-secret",
		"BOLTE_BRIDGE_MATRIX_HS_TOKEN":         "hs-secret",
		"BOLTE_BRIDGE_MATRIX_SENDER_LOCALPART": "bolte",
		"BOLTE_BRIDGE_MATRIX_ROOM_ID":          "!room:matrix.example.org",
	}

	tests := []struct {
		name string
		args []string
		env  map[string]string
		want matrix.Config
	}{
		{
			name: "all fields resolved from the environment",
			env:  base,
			want: matrix.Config{
				HomeserverURL:   "https://matrix.example.org",
				ServerName:      "matrix.example.org",
				AppServiceID:    "bolte-bridge",
				ASToken:         "as-secret",
				HSToken:         "hs-secret",
				SenderLocalpart: "bolte",
				RoomID:          "!room:matrix.example.org",
			},
		},
		{
			name: "flags override the environment for non-secret fields",
			args: []string{
				"--matrix-homeserver-url", "https://flag.example.org",
				"--matrix-server-name", "flag.example.org",
				"--matrix-appservice-id", "flag-id",
				"--matrix-sender-localpart", "flagbot",
				"--matrix-room-id", "!flag:flag.example.org",
			},
			env: base,
			want: matrix.Config{
				HomeserverURL:   "https://flag.example.org",
				ServerName:      "flag.example.org",
				AppServiceID:    "flag-id",
				ASToken:         "as-secret",
				HSToken:         "hs-secret",
				SenderLocalpart: "flagbot",
				RoomID:          "!flag:flag.example.org",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg, err := Load(tc.args, matrixSection)
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			if cfg.Matrix != tc.want {
				t.Errorf("cfg.Matrix = %+v, want %+v", cfg.Matrix, tc.want)
			}
		})
	}
}

func TestLoadMatrixValidationErrorsAbort(t *testing.T) {
	// Every matrix field is required, so leaving any one empty must abort Load
	// with an error naming the offending key. Each case starts from a fully
	// populated environment and clears exactly one variable; setting it to the
	// empty string also guards against an ambient value leaking in from the test
	// host. The env is authoritative for all fields, including the non-secret
	// ones, so no flags are needed to exercise the validation paths.
	full := map[string]string{
		"BOLTE_BRIDGE_MATRIX_HOMESERVER_URL":   "https://matrix.example.org",
		"BOLTE_BRIDGE_MATRIX_SERVER_NAME":      "matrix.example.org",
		"BOLTE_BRIDGE_MATRIX_APPSERVICE_ID":    "bolte-bridge",
		"BOLTE_BRIDGE_MATRIX_AS_TOKEN":         "as-secret",
		"BOLTE_BRIDGE_MATRIX_HS_TOKEN":         "hs-secret",
		"BOLTE_BRIDGE_MATRIX_SENDER_LOCALPART": "bolte",
		"BOLTE_BRIDGE_MATRIX_ROOM_ID":          "!room:matrix.example.org",
	}

	tests := []struct {
		name    string
		empty   string // the env variable to clear
		wantKey string // the dotted key the error must name
	}{
		{"empty homeserver-url", "BOLTE_BRIDGE_MATRIX_HOMESERVER_URL", "matrix.homeserver-url"},
		{"empty server-name", "BOLTE_BRIDGE_MATRIX_SERVER_NAME", "matrix.server-name"},
		{"empty appservice-id", "BOLTE_BRIDGE_MATRIX_APPSERVICE_ID", "matrix.appservice-id"},
		{"empty as-token", "BOLTE_BRIDGE_MATRIX_AS_TOKEN", "matrix.as-token"},
		{"empty hs-token", "BOLTE_BRIDGE_MATRIX_HS_TOKEN", "matrix.hs-token"},
		{
			"empty sender-localpart",
			"BOLTE_BRIDGE_MATRIX_SENDER_LOCALPART",
			"matrix.sender-localpart",
		},
		{"empty room-id", "BOLTE_BRIDGE_MATRIX_ROOM_ID", "matrix.room-id"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range full {
				if k == tc.empty {
					v = ""
				}
				t.Setenv(k, v)
			}

			_, err := Load(nil, matrixSection)
			if err == nil {
				t.Fatalf("Load with empty %s returned nil error, want non-nil", tc.wantKey)
			}
			if !strings.Contains(err.Error(), tc.wantKey) {
				t.Errorf("error %q does not mention the offending key %s", err, tc.wantKey)
			}
		})
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
