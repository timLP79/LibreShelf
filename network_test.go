// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"testing"
)

func TestIsExternalAllowed_EnvDefaultTrue_NoSettingsRow(t *testing.T) {
	dm := setupTestDB(t)
	if !isExternalAllowedFn(dm, false /* offlineFromEnv */) {
		t.Errorf("env-default online, no settings row: want allowed=true, got false")
	}
}

func TestIsExternalAllowed_EnvDefaultOffline_NoSettingsRow(t *testing.T) {
	dm := setupTestDB(t)
	if isExternalAllowedFn(dm, true /* offlineFromEnv */) {
		t.Errorf("env-default offline, no settings row: want allowed=false, got true")
	}
}

func TestIsExternalAllowed_SettingsOverridesEnvToOffline(t *testing.T) {
	dm := setupTestDB(t)
	adminID := mustCreateUser(t, dm, "admin_z", "admin")
	if err := dm.SetSetting("offline_mode", "true", adminID); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if isExternalAllowedFn(dm, false /* env says online */) {
		t.Errorf("settings=true must override env=false: want allowed=false, got true")
	}
}

func TestIsExternalAllowed_SettingsOverridesEnvToOnline(t *testing.T) {
	dm := setupTestDB(t)
	adminID := mustCreateUser(t, dm, "admin_y", "admin")
	if err := dm.SetSetting("offline_mode", "false", adminID); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if !isExternalAllowedFn(dm, true /* env says offline */) {
		t.Errorf("settings=false must override env=true: want allowed=true, got false")
	}
}

func TestIsExternalAllowed_DBErrorFallsBackToEnvDefault(t *testing.T) {
	// Simulate a DB fault: open a DM, then immediately close its
	// underlying *sql.DB so every subsequent query returns
	// "sql: database is closed". The predicate must fall back to
	// the env-var default and not panic.
	dm := NewDatabaseManager(t.TempDir() + "/broken.sqlite")
	if err := dm.db.Close(); err != nil {
		t.Fatalf("close dm.db: %v", err)
	}

	if !isExternalAllowedFn(dm, false /* env=online */) {
		t.Errorf("DB error + env=online: want allowed=true (fallback to env), got false")
	}
	if isExternalAllowedFn(dm, true /* env=offline */) {
		t.Errorf("DB error + env=offline: want allowed=false (fallback to env), got true")
	}
}
