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

// offlineEnvDefault is the startup value of LIBRESHELF_OFFLINE.
// Captured once so the predicate does not re-read os.Getenv on every
// call. Settings table overrides this when a row is present.
var offlineEnvDefault bool

func init() {
	offlineEnvDefault = strings.EqualFold(os.Getenv("LIBRESHELF_OFFLINE"), "true")
}

// IsExternalAllowed returns true when external HTTP calls are
// permitted for this deployment. Reads the offline_mode row from the
// settings table if present; otherwise uses the LIBRESHELF_OFFLINE
// env-var default captured at startup.
func IsExternalAllowed(dm *DatabaseManager) bool {
	return isExternalAllowedFn(dm, offlineEnvDefault)
}

// isExternalAllowedFn is the testable inner. Tests inject the
// offlineFromEnv value directly so they do not have to manipulate
// process env vars.
func isExternalAllowedFn(dm *DatabaseManager, offlineFromEnv bool) bool {
	v, err := dm.GetSetting("offline_mode")
	if err != nil {
		log.Printf("IsExternalAllowed: GetSetting failed, falling back to env default: %v", err)
		return !offlineFromEnv
	}
	if v == "" {
		return !offlineFromEnv
	}
	return !strings.EqualFold(v, "true")
}
