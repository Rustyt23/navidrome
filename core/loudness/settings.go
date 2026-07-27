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

var (
	mu     sync.RWMutex
	cached *bool
)

// Enabled reports whether loudness normalization is currently enabled.
//
// The value set from the UI (stored in the property table) takes precedence
// over Scanner.LoudnessNormalization.Enabled in the configuration file. The
// configured value is only used until the toggle has been used for the first
// time, so an existing navidrome.toml keeps working unchanged.
func Enabled(ctx context.Context, ds model.DataStore) bool {
	mu.RLock()
	value := cached
	mu.RUnlock()
	if value != nil {
		return *value
	}

	enabled := conf.Server.Scanner.LoudnessNormalization.Enabled
	if ds == nil {
		return enabled
	}

	stored, err := ds.Property(ctx).Get(consts.LoudnessNormalizationEnabledKey)
	switch {
	case errors.Is(err, model.ErrNotFound):
		// Never toggled from the UI: keep following the configuration file
		// instead of caching, so config reloads are picked up.
		return enabled
	case err != nil:
		log.Warn(ctx, "Could not read loudness normalization setting, using configured value", "enabled", enabled, err)
		return enabled
	}

	parsed, err := strconv.ParseBool(stored)
	if err != nil {
		log.Warn(ctx, "Invalid stored loudness normalization setting, using configured value", "value", stored, "enabled", enabled, err)
		return enabled
	}

	mu.Lock()
	cached = &parsed
	mu.Unlock()
	return parsed
}

// SetEnabled persists the on/off state chosen in the UI.
func SetEnabled(ctx context.Context, ds model.DataStore, enabled bool) error {
	if err := ds.Property(ctx).Put(consts.LoudnessNormalizationEnabledKey, strconv.FormatBool(enabled)); err != nil {
		return err
	}
	mu.Lock()
	cached = &enabled
	mu.Unlock()
	return nil
}

// ResetCache drops the in-memory copy of the stored setting. Used by tests.
func ResetCache() {
	mu.Lock()
	cached = nil
	mu.Unlock()
}
