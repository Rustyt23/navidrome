package gcpsync

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

// namingEnforcement is the UI-controlled switch for the naming gate. Stored in
// the property table so it survives restarts and so the server - which is what
// actually blocks the upload - can read it. The value set from the UI takes
// precedence over the configuration file; the configured value is used only
// until the toggle has been used for the first time.
var namingEnforcement = struct {
	mu     sync.RWMutex
	cached *bool
	ds     model.DataStore
}{}

// SetDataStore hands the service the store it needs to read UI-controlled
// settings. Called once at startup; without it the gate follows the config
// file alone.
func SetDataStore(ds model.DataStore) {
	namingEnforcement.mu.Lock()
	namingEnforcement.ds = ds
	namingEnforcement.mu.Unlock()
}

// NamingEnforced reports whether files not named "Artist - Title.mp3" are
// currently blocked from the bucket.
func NamingEnforced(ctx context.Context) bool {
	configured := conf.Server.GCPSync.EnforceNaming

	namingEnforcement.mu.RLock()
	value, ds := namingEnforcement.cached, namingEnforcement.ds
	namingEnforcement.mu.RUnlock()

	if value != nil {
		return *value
	}
	if ds == nil {
		return configured
	}

	stored, err := ds.Property(ctx).Get(consts.GCPSyncNamingEnforcementKey)
	switch {
	case errors.Is(err, model.ErrNotFound):
		// Never toggled from the UI: keep following the configuration file
		// instead of caching, so config reloads are picked up.
		return configured
	case err != nil:
		log.Warn(ctx, "Could not read naming enforcement setting, using configured value",
			"configured", configured, err)
		return configured
	}

	parsed, err := strconv.ParseBool(stored)
	if err != nil {
		log.Warn(ctx, "Invalid stored naming enforcement setting, using configured value",
			"value", stored, "configured", configured, err)
		return configured
	}

	namingEnforcement.mu.Lock()
	namingEnforcement.cached = &parsed
	namingEnforcement.mu.Unlock()
	return parsed
}

// SetNamingEnforced persists the on/off state chosen in the UI.
func SetNamingEnforced(ctx context.Context, ds model.DataStore, enabled bool) error {
	if err := ds.Property(ctx).Put(consts.GCPSyncNamingEnforcementKey, strconv.FormatBool(enabled)); err != nil {
		return err
	}
	namingEnforcement.mu.Lock()
	namingEnforcement.cached = &enabled
	namingEnforcement.mu.Unlock()
	return nil
}

// ResetNamingCache drops the in-memory copy of the stored setting. Used by tests.
func ResetNamingCache() {
	namingEnforcement.mu.Lock()
	namingEnforcement.cached = nil
	namingEnforcement.mu.Unlock()
}
