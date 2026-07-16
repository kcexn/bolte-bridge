// Package config assembles the bridge's runtime configuration from command-line
// arguments and environment variables into a single Config value.
//
// Configuration is composed from independent sections (see SectionFunc). Each
// section owns one slice of the settings surface by declaring its own flags and
// environment bindings, then folds the resolved values into Config.
//
// Resolution precedence, handled by Viper, is: command-line flag, then
// environment variable, then built-in default.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"bolte-bridge/internal/store"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// envPrefix namespaces every environment variable the bridge reads. A binder
// key such as "db.path" is looked up in the environment as BOLTE_BRIDGE_DB_PATH.
const envPrefix = "BOLTE_BRIDGE"

// envReplacer rewrites a dotted, hyphenated config key into the flat,
// upper-snake form used in the environment: "db.path" and "db-path" both become
// "DB_PATH", which SetEnvPrefix then prepends with BOLTE_BRIDGE_.
var envReplacer = strings.NewReplacer(".", "_", "-", "_")

var (
	ErrHelp             = errors.New("help requested")
	ErrInvalidArguments = errors.New("invalid arguments")
)

// Config is the unified configuration for all application components. It is
// composed of one nested block per concern, each populated by its own section
// (see sections.go).
type Config struct {
	// Store holds the state-store settings.
	Store store.Config
}

// ApplyFunc folds a section's resolved values into the shared Config. It runs
// once, after argv and the environment have been parsed, and may validate what
// it reads: returning an error aborts the whole build.
type ApplyFunc func(cfg *Config) error

// SectionFunc is one self-contained unit of configuration. It binds flags
// and environment variables to Viper keys, then returns an ApplyFunc
// that Load uses to construct a Config object after all supplied command-line
// arguments have been parsed.
type SectionFunc func(b *Binder) (ApplyFunc, error)

// Load builds a Config from args (typically os.Args[1:]) and the environment,
// using the given sections. It binds every section's flags onto a single flag
// set, parses args, then runs each section's ApplyFunc in order. Flags override
// environment variables, which override defaults.
//
// If the supplied arguments request help (for example, via -h or --help),
// Load prints the usage message and returns ErrHelp.
// If argument parsing fails, Load prints the parse error and usage message, then
// returns ErrInvalidArguments.
func Load(args []string, sections ...SectionFunc) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	// Map dotted config keys onto the flat, underscore-delimited environment
	// namespace: key "db.path" resolves to BOLTE_BRIDGE_DB_PATH.
	v.SetEnvKeyReplacer(envReplacer)

	fs := pflag.NewFlagSet("bolte-bridge", pflag.ContinueOnError)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "Bolte Bridge\n\n")
		_, _ = fmt.Fprintf(fs.Output(), "Usage:\n")
		_, _ = fmt.Fprintf(fs.Output(), "  bolte-bridge [OPTIONS]\n\n")
		_, _ = fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
	}

	b := &Binder{v: v, fs: fs}

	applies := make([]ApplyFunc, 0, len(sections))
	for _, section := range sections {
		apply, err := section(b)
		if err != nil {
			return nil, fmt.Errorf("config: bind section: %w", err)
		}
		applies = append(applies, apply)
	}

	if err := fs.Parse(args); err != nil {
		if err == pflag.ErrHelp {
			return nil, ErrHelp
		}
		fmt.Fprintln(os.Stderr, err)
		fs.Usage()
		return nil, ErrInvalidArguments
	}

	cfg := &Config{}
	for _, apply := range applies {
		if err := apply(cfg); err != nil {
			return nil, fmt.Errorf("config: apply section: %w", err)
		}
	}
	return cfg, nil
}
