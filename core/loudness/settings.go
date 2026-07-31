// Package loudness holds the runtime state for loudness (LUFS) normalization
// that is shared between the scanner, the native API and the UI.
package loudness

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// One toggle's worth of state: the value stored from the UI, once read.
type storedSetting struct {
	mu     sync.RWMutex
	cached *bool
	key    string
	name   string
}

var (
	enabledSetting = &storedSetting{key: consts.LoudnessNormalizationEnabledKey, name: "loudness normalization"}
	backupSetting  = &storedSetting{key: consts.LoudnessNormalizationBackupKey, name: "loudness backup"}
)

// get returns the stored value, falling back to the configured one.
//
// The value set from the UI (stored in the property table) takes precedence
// over the configuration file. The configured value is only used until the
// toggle has been used for the first time, so an existing navidrome.toml keeps
// working unchanged.
func (s *storedSetting) get(ctx context.Context, ds model.DataStore, configured bool) bool {
	s.mu.RLock()
	value := s.cached
	s.mu.RUnlock()
	if value != nil {
		return *value
	}
	if ds == nil {
		return configured
	}

	stored, err := ds.Property(ctx).Get(s.key)
	switch {
	case errors.Is(err, model.ErrNotFound):
		// Never toggled from the UI: keep following the configuration file
		// instead of caching, so config reloads are picked up.
		return configured
	case err != nil:
		log.Warn(ctx, "Could not read "+s.name+" setting, using configured value", "configured", configured, err)
		return configured
	}

	parsed, err := strconv.ParseBool(stored)
	if err != nil {
		log.Warn(ctx, "Invalid stored "+s.name+" setting, using configured value",
			"value", stored, "configured", configured, err)
		return configured
	}

	s.mu.Lock()
	s.cached = &parsed
	s.mu.Unlock()
	return parsed
}

func (s *storedSetting) set(ctx context.Context, ds model.DataStore, value bool) error {
	if err := ds.Property(ctx).Put(s.key, strconv.FormatBool(value)); err != nil {
		return err
	}
	s.mu.Lock()
	s.cached = &value
	s.mu.Unlock()
	return nil
}

func (s *storedSetting) reset() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

// Enabled reports whether loudness normalization is currently enabled.
func Enabled(ctx context.Context, ds model.DataStore) bool {
	return enabledSetting.get(ctx, ds, conf.Server.Scanner.LoudnessNormalization.Enabled)
}

// SetEnabled persists the on/off state chosen in the UI.
func SetEnabled(ctx context.Context, ds model.DataStore, enabled bool) error {
	return enabledSetting.set(ctx, ds, enabled)
}

// BackupEnabled reports whether an untouched original is kept before a song is
// rewritten.
//
// Turning this off is the one setting here that cannot be undone: without a
// stored original there is nothing to restore from, and no way to prove after
// the fact that only the level changed. It exists because keeping a copy of
// every song costs about as much disk as the library itself, which a client who
// already holds the masters elsewhere may not want to pay twice.
func BackupEnabled(ctx context.Context, ds model.DataStore) bool {
	return backupSetting.get(ctx, ds, conf.Server.Scanner.LoudnessNormalization.Backup)
}

// SetBackupEnabled persists the on/off state chosen in the UI.
func SetBackupEnabled(ctx context.Context, ds model.DataStore, enabled bool) error {
	return backupSetting.set(ctx, ds, enabled)
}

// ResetCache drops the in-memory copies of the stored settings. Used by tests.
func ResetCache() {
	enabledSetting.reset()
	backupSetting.reset()
}
