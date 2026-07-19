package config

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Binder is the surface a SectionFunc uses to declare its configuration. Each
// helper wires one setting across all three sources at once — it registers a
// flag on the shared flag set, binds that flag and its environment variable to a
// Viper key, and records the default — so a section states each setting exactly
// once.
//
// A key is a dotted path, e.g. "db.path". The flag name is the caller's choice
// (conventionally the key with dots as hyphens); the environment variable is
// derived from the key by the prefix and replacer configured in Load.
type Binder struct {
	v  *viper.Viper
	fs *pflag.FlagSet
}

// StringP registers a string setting: a --flag with an optional -f shorthand,
// defaulting to fallback.
func (b *Binder) StringP(key string, long string, short string, fallback string, usage string) {
	b.fs.StringP(long, short, fallback, usage)
	b.bind(key, long, fallback)
}

// String registers a string setting with a --flag but no shorthand. Use it when
// there is no natural single-letter alias to reserve.
func (b *Binder) String(key string, long string, fallback string, usage string) {
	b.StringP(key, long, "", fallback, usage)
}

// Secret registers a string setting that is resolvable only from the
// environment (and its default), with no command-line flag. Use it for
// sensitive values such as passwords, which should not appear in argv where
// they would be visible in the process table and shell history.
func (b *Binder) Secret(key string, fallback string) {
	_ = b.v.BindEnv(key)
	b.v.SetDefault(key, fallback)
}

// Viper returns the underlying Viper instance so an ApplyFunc can read resolved
// values by key (GetString, GetBool, GetInt, GetDuration, ...).
func (b *Binder) Viper() *viper.Viper {
	return b.v
}

// bind connects a registered flag and its environment variable to key and
// records the default. Viper uses the flag's value only when the flag was set
// on the command line, so precedence is flag, then environment, then default.
func (b *Binder) bind(key, flag string, fallback any) {
	_ = b.v.BindPFlag(key, b.fs.Lookup(flag))
	_ = b.v.BindEnv(key)
	b.v.SetDefault(key, fallback)
}
