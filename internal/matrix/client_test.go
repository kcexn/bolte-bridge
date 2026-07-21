package matrix

import (
	"strings"
	"testing"
)

// validConfig returns a Config with every required field populated, suitable
// for mutating in tests that want to omit exactly one field.
func validConfig() Config {
	return Config{
		HomeserverURL:   "https://matrix.example.org",
		ServerName:      "example.org",
		AppServiceID:    "bolte-bridge",
		ASToken:         "as-token",
		HSToken:         "hs-token",
		SenderLocalpart: "bolte",
		RoomID:          "!room:example.org",
	}
}

func TestConfigValidateValid(t *testing.T) {
	if err := validConfig().validate(); err != nil {
		t.Fatalf("validate() on a fully populated Config returned error: %v", err)
	}
}

func TestConfigValidateInvalid(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantMsg string
	}{
		{
			name:    "missing HomeserverURL",
			mutate:  func(c *Config) { c.HomeserverURL = "" },
			wantMsg: "matrix: Config.HomeserverURL is required",
		},
		{
			name:    "missing ServerName",
			mutate:  func(c *Config) { c.ServerName = "" },
			wantMsg: "matrix: Config.ServerName is required",
		},
		{
			name:    "missing AppServiceID",
			mutate:  func(c *Config) { c.AppServiceID = "" },
			wantMsg: "matrix: Config.AppServiceID is required",
		},
		{
			name:    "missing ASToken",
			mutate:  func(c *Config) { c.ASToken = "" },
			wantMsg: "matrix: Config.ASToken is required",
		},
		{
			name:    "missing HSToken",
			mutate:  func(c *Config) { c.HSToken = "" },
			wantMsg: "matrix: Config.HSToken is required",
		},
		{
			name:    "missing SenderLocalpart",
			mutate:  func(c *Config) { c.SenderLocalpart = "" },
			wantMsg: "matrix: Config.SenderLocalpart is required",
		},
		{
			name:    "missing RoomID",
			mutate:  func(c *Config) { c.RoomID = "" },
			wantMsg: "matrix: Config.RoomID is required",
		},
		{
			name:    "empty Config reports the first field",
			mutate:  func(c *Config) { *c = Config{} },
			wantMsg: "matrix: Config.HomeserverURL is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)

			err := cfg.validate()
			if err == nil {
				t.Fatalf("validate() returned nil, want error %q", tc.wantMsg)
			}
			if got := err.Error(); got != tc.wantMsg {
				t.Errorf("validate() error = %q, want %q", got, tc.wantMsg)
			}
			if !strings.HasPrefix(err.Error(), "matrix:") {
				t.Errorf("validate() error %q is missing the package prefix", err.Error())
			}
		})
	}
}
