// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"testing"
)

// withOfflineEnvDefault swaps offlineEnvDefault for the duration of the
// test function and restores the prior value on Cleanup. Tests that
// need to exercise the env-locked branches use this instead of
// os.Setenv, which would not retroactively change the package var
// since init() ran at process start.
func withOfflineEnvDefault(t *testing.T, locked bool) {
	t.Helper()
	prior := offlineEnvDefault
	offlineEnvDefault = locked
	t.Cleanup(func() { offlineEnvDefault = prior })
}

func TestIsExternalAllowed_EnvDefaultTrue_NoSettingsRow(t *testing.T) {
	dm := setupTestDB(t)
	withOfflineEnvDefault(t, true)
	if IsExternalAllowed(dm) {
		t.Errorf("env locked, no settings row: want allowed=false, got true")
	}
}

func TestIsExternalAllowed_EnvDefaultFalse_NoSettingsRow(t *testing.T) {
	dm := setupTestDB(t)
	withOfflineEnvDefault(t, false)
	if !IsExternalAllowed(dm) {
		t.Errorf("env not locked, no settings row: want allowed=true, got false")
	}
}

func TestIsExternalAllowed_SettingsOverridesEnvToOffline(t *testing.T) {
	dm := setupTestDB(t)
	adminID := mustCreateUser(t, dm, "admin_z", "admin")
	if err := dm.SetSetting("offline_mode", "true", adminID); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	withOfflineEnvDefault(t, false)
	if IsExternalAllowed(dm) {
		t.Errorf("env unlocked, DB=true: want allowed=false (DB wins), got true")
	}
}

func TestIsExternalAllowed_EnvLockBeatsDBOnline(t *testing.T) {
	dm := setupTestDB(t)
	adminID := mustCreateUser(t, dm, "admin_y", "admin")
	if err := dm.SetSetting("offline_mode", "false", adminID); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	withOfflineEnvDefault(t, true)
	if IsExternalAllowed(dm) {
		t.Errorf("env locked, DB=false: want allowed=false (lock wins over DB), got true")
	}
}

func TestIsExternalAllowed_DBErrorWhenUnlockedFallsBackToAllow(t *testing.T) {
	dm := NewDatabaseManager(t.TempDir() + "/broken.sqlite")
	if err := dm.db.Close(); err != nil {
		t.Fatalf("close dm.db: %v", err)
	}
	withOfflineEnvDefault(t, false)
	if !IsExternalAllowed(dm) {
		t.Errorf("DB error + env unlocked: want allowed=true (fail-open), got false")
	}
}

func TestIsExternalAllowed_DBErrorWhenLockedStaysBlocked(t *testing.T) {
	dm := NewDatabaseManager(t.TempDir() + "/broken.sqlite")
	if err := dm.db.Close(); err != nil {
		t.Fatalf("close dm.db: %v", err)
	}
	withOfflineEnvDefault(t, true)
	if IsExternalAllowed(dm) {
		t.Errorf("DB error + env locked: want allowed=false (lock fires before DB read), got true")
	}
}

func TestIsOfflineEnvLocked_ReturnsEnvDefault(t *testing.T) {
	withOfflineEnvDefault(t, true)
	if !IsOfflineEnvLocked() {
		t.Errorf("locked=true: want true, got false")
	}
	withOfflineEnvDefault(t, false)
	if IsOfflineEnvLocked() {
		t.Errorf("locked=false: want false, got true")
	}
}
