// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"errors"
	"log"
	"os"
	"strings"
)

// ErrExternalDisabled is returned by external-API entry points when
// offline mode is on. Callers should treat this as a non-error
// short-circuit: render a friendly message, do not log as an upstream
// failure.
var ErrExternalDisabled = errors.New("external API calls disabled (offline mode)")

// offlineEnvDefault is the startup value of LIBRESHELF_OFFLINE,
// captured once at process start. IsExternalAllowed checks it as a
// lock guard; IsOfflineEnvLocked exposes it to the admin UI. Tests
// override it via withOfflineEnvDefault.
var offlineEnvDefault bool

func init() {
	offlineEnvDefault = strings.EqualFold(os.Getenv("LIBRESHELF_OFFLINE"), "true")
}

// IsExternalAllowed returns true when external HTTP calls are
// permitted for this deployment. When LIBRESHELF_OFFLINE=true is set
// at startup, the env var acts as a deployment lock: it overrides any
// DB row. Otherwise the offline_mode row in the settings table
// controls; default true (external HTTP allowed).
func IsExternalAllowed(dm *DatabaseManager) bool {
	if offlineEnvDefault {
		return false
	}
	return isExternalAllowedFromDB(dm)
}

// isExternalAllowedFromDB reads the offline_mode settings row.
// Called only by IsExternalAllowed when the env var is not locking;
// the lock check is the caller's responsibility. On a DB read error
// returns true (fail-open).
func isExternalAllowedFromDB(dm *DatabaseManager) bool {
	v, err := dm.GetSetting("offline_mode")
	if err != nil {
		log.Printf("IsExternalAllowed: GetSetting failed, defaulting to allow: %v", err)
		return true
	}
	if v == "" {
		return true
	}
	return !strings.EqualFold(v, "true")
}

// IsOfflineEnvLocked returns true when the LIBRESHELF_OFFLINE env var
// is locking offline mode (set to "true" at startup). The admin
// Settings page uses this to render the toggle as locked.
func IsOfflineEnvLocked() bool {
	return offlineEnvDefault
}
