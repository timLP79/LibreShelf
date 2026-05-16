# Offline Mode Env-Var-as-Lock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Flip the offline-mode precedence so `LIBRESHELF_OFFLINE=true` at startup overrides the DB row instead of the other way around. Admin Settings UI shows the toggle as locked; handler skips writing the offline_mode row while locked.

**Architecture:** Three changes layered. (1) `network.go` predicate gains an env-lock check that short-circuits the DB read, plus a new exported `IsOfflineEnvLocked()` accessor; old two-arg testable inner is replaced by a one-arg `isExternalAllowedFromDB(dm)`. (2) Settings handler reads the lock state to gate writes to the `offline_mode` row; template branches on the lock state for the toggle markup and help copy. (3) `main.go` logs a single line at startup when locked.

**Tech Stack:** Go 1.25.9+, Gin, html/template, SQLite via modernc.org/sqlite, existing test helpers `setupTestRouter` + `loginAs` + `setupTestDB` + `mustCreateUser`.

**Spec:** `docs/specs/2026-05-15-offline-mode-env-lock-design.md`

---

## File Structure

- **Modify** `network.go` -- rewrite `IsExternalAllowed`, replace inner `isExternalAllowedFn(dm, offlineFromEnv bool)` with `isExternalAllowedFromDB(dm)`, add `IsOfflineEnvLocked()`. The package var `offlineEnvDefault` and `init()` stay unchanged.
- **Modify** `network_test.go` -- add `withOfflineEnvDefault(t, locked)` helper, rewrite all five existing predicate tests to use the helper + `IsExternalAllowed(dm)` directly, add new tests for env-lock-beats-DB, DB-error-when-locked, and the `IsOfflineEnvLocked` accessor.
- **Modify** `handlers_settings.go` -- `HandleSettings` adds `OfflineModeLocked` to the template map and changes the offline_mode default from `offlineEnvDefault` to `false`. `HandleSettingsPost` loop entries gain a `skipWrite` field that gates the offline_mode write on `IsOfflineEnvLocked()`.
- **Modify** `templates/admin_settings.html` -- the External API access card body wraps `checked disabled` and help-text content in a `{{if .Settings.OfflineModeLocked}}...{{else}}...{{end}}` branch.
- **Modify** `handlers_settings_test.go` -- add `TestSettingsPageGET_RendersLockedToggleWhenEnvLocked` and `TestSettingsPagePOST_SkipsOfflineModeWhenEnvLocked`. Existing tests unchanged.
- **Modify** `main.go` -- add a single `log.Printf` call right after `dm.SeedDefaultUsers()` when `IsOfflineEnvLocked()` returns true.
- **Modify** `DECISIONS.md` -- add a one-line "Superseded in part by DEC-034" callout to DEC-033; append DEC-034.
- **Modify** `docs/deployment.md` -- rewrite the `LIBRESHELF_OFFLINE` subsection to match the new lock semantics.

---

## Task 1: Predicate rewrite + predicate test rewrite

This is a single TDD cycle: the API of `isExternalAllowedFn` changes, so tests and implementation must move together (the package would not compile otherwise).

**Files:**
- Modify: `network.go`
- Modify: `network_test.go`

- [ ] **Step 1: Replace `network_test.go` contents with the new test suite**

The complete new file body (preserve the existing copyright header):

```go
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
```

- [ ] **Step 2: Run tests, verify compile failures**

Run: `go test ./... -run TestIsExternalAllowed 2>&1 | head -20`

Expected: compile errors. `IsOfflineEnvLocked` undefined. `isExternalAllowedFn` no longer referenced but still exists in the source -- that is fine, it just becomes dead code until the next step replaces it. The key failures are around `IsOfflineEnvLocked` not existing.

- [ ] **Step 3: Rewrite `network.go`**

The complete new file body (preserve the copyright header):

```go
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
// Captured once so IsOfflineEnvLocked does not re-read os.Getenv on
// every call. Tests override this via withOfflineEnvDefault.
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

// isExternalAllowedFromDB reads the offline_mode settings row. Used
// by IsExternalAllowed when the env var is not locking, and by tests
// that want to exercise the DB-path branches without invoking
// withOfflineEnvDefault.
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
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./... -run TestIsExternalAllowed -v`
Expected: 6 PASS (the five renamed/rewritten tests plus the new env-lock-beats-DB-online test).

Run: `go test ./... -run TestIsOfflineEnvLocked -v`
Expected: 1 PASS.

Run full suite to confirm no regressions in callers of `IsExternalAllowed` (handlers, seed backfill):
Run: `go test ./...`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add network.go network_test.go
git commit -m "$(cat <<'EOF'
feat(network): flip offline-mode precedence; env var is now a lock

LIBRESHELF_OFFLINE=true at startup overrides any DB row. The two-arg
testable inner isExternalAllowedFn is replaced by isExternalAllowedFromDB
(no env arg needed; env lock is checked in IsExternalAllowed before the
DB read). New IsOfflineEnvLocked accessor for the admin UI to render the
locked state.

DB-error policy: when env is locking, the lock fires before the DB
read so a DB fault cannot accidentally flip the deployment online.
When env is not locking, a DB fault falls back to allow (fail-open).

Refs cs408-go-stack-85j.
EOF
)"
```

---

## Task 2: Handler + template change

Handler test expectations depend on the template rendering "disabled" and the lock copy. Handler and template must change together. Single TDD cycle, single commit.

**Files:**
- Modify: `handlers_settings.go`
- Modify: `templates/admin_settings.html`
- Modify: `handlers_settings_test.go`

- [ ] **Step 1: Append new handler tests to `handlers_settings_test.go`**

Append at the end of the file:

```go
func TestSettingsPageGET_RendersLockedToggleWhenEnvLocked(t *testing.T) {
	router, dm := setupTestRouter(t)
	withOfflineEnvDefault(t, true)
	sess, _ := loginAs(t, dm, "admin1", "admin")

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "disabled") {
		t.Errorf("expected 'disabled' attribute in locked toggle, got %q", body)
	}
	if !strings.Contains(body, "LIBRESHELF_OFFLINE") {
		t.Errorf("expected 'LIBRESHELF_OFFLINE' explanation in body, got %q", body)
	}
}

func TestSettingsPagePOST_SkipsOfflineModeWhenEnvLocked(t *testing.T) {
	router, dm := setupTestRouter(t)
	withOfflineEnvDefault(t, true)
	sess, csrf := loginAs(t, dm, "admin1", "admin")

	// Pre-condition: no offline_mode row.
	if v, _ := dm.GetSetting("offline_mode"); v != "" {
		t.Fatalf("pre-condition: offline_mode row should be empty, got %q", v)
	}

	// Submit a crafted POST that tries to flip offline_mode to true
	// while the env-var lock is in place. The handler must skip the
	// write regardless of the form value. Also flip the staff toggle
	// so we can confirm OTHER settings still write normally.
	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("offline_mode", "on")
	form.Set("staff_can_import_patrons", "on")
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}

	// Post-condition: offline_mode row still empty (write skipped).
	v, err := dm.GetSetting("offline_mode")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if v != "" {
		t.Errorf("offline_mode row should not be written while locked, got %q", v)
	}

	// Confirm staff_can_import_patrons WAS written (lock affects only offline_mode).
	if !dm.GetSettingBool("staff_can_import_patrons", false) {
		t.Errorf("staff_can_import_patrons should be true (lock only affects offline_mode)")
	}
}
```

No new imports needed; `net/http`, `net/http/httptest`, `net/url`, `strings`, `testing` are all present from prior tests in the file.

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./... -run "TestSettingsPageGET_RendersLockedToggleWhenEnvLocked|TestSettingsPagePOST_SkipsOfflineModeWhenEnvLocked" -v`

Expected: both FAIL.

- GET test fails because the template does not yet branch on `OfflineModeLocked` and renders the unlocked variant.
- POST test fails because `HandleSettingsPost` still writes every key on every POST regardless of the lock state.

- [ ] **Step 3: Update `handlers_settings.go`**

Replace BOTH handler bodies. The complete new file:

```go
// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleSettings(c *gin.Context) {
	dm := getDB(c)
	renderTemplate(c, "admin_settings", gin.H{
		"Title":   "Settings",
		"Error":   readAndClearFlash(c, flashKindError),
		"Success": readAndClearFlash(c, flashKindSuccess),
		"Settings": gin.H{
			"StaffCanImportPatrons": dm.GetSettingBool("staff_can_import_patrons", false),
			"OfflineMode":           dm.GetSettingBool("offline_mode", false),
			"OfflineModeLocked":     IsOfflineEnvLocked(),
		},
	})
}

func HandleSettingsPost(c *gin.Context) {
	user := c.MustGet("user").(*User)
	dm := getDB(c)

	type setting struct {
		key       string
		field     string
		skipWrite bool
	}
	for _, s := range []setting{
		{"staff_can_import_patrons", "staff_can_import_patrons", false},
		{"offline_mode", "offline_mode", IsOfflineEnvLocked()},
	} {
		if s.skipWrite {
			continue
		}
		value := "false"
		if c.PostForm(s.field) == "on" {
			value = "true"
		}
		if err := dm.SetSetting(s.key, value, user.ID); err != nil {
			log.Printf("settings save failed for %s: %v", s.key, err)
			setFlash(c, flashKindError, "settings_save_failed")
			c.Redirect(http.StatusSeeOther, "/admin/settings")
			return
		}
	}

	setFlash(c, flashKindSuccess, "settings_saved")
	c.Redirect(http.StatusSeeOther, "/admin/settings")
}
```

Two changes from the prior version:
1. `HandleSettings` adds `OfflineModeLocked: IsOfflineEnvLocked()` and changes the offline_mode default from `offlineEnvDefault` to `false`.
2. `HandleSettingsPost` adds a `skipWrite bool` field on the per-setting struct and a `continue` when it's true.

- [ ] **Step 4: Update `templates/admin_settings.html`**

Locate the "External API access" `<section>` card. Replace the inner `<div class="form-check form-switch">...</div>` block with:

```html
            <div class="form-check form-switch">
                <input class="form-check-input" type="checkbox" role="switch"
                       id="offline_mode" name="offline_mode"
                       {{if .Settings.OfflineModeLocked}}checked disabled{{else if .Settings.OfflineMode}}checked{{end}}>
                <label class="form-check-label" for="offline_mode">
                    <strong>Offline mode (disable external API calls)</strong>
                </label>
                <div class="form-text small">
                    {{if .Settings.OfflineModeLocked}}
                        Locked by the <code>LIBRESHELF_OFFLINE</code> env var.
                        Clear the env var and restart the server to edit this
                        setting from the UI.
                    {{else}}
                        When enabled, LibreShelf will not contact Open Library
                        (or future external metadata sources) for cover art
                        or book information. Book entry continues to work
                        using whatever information staff type in manually.
                        Use this for deployments without internet access or
                        where outbound HTTP is policy-restricted.
                    {{end}}
                </div>
            </div>
```

Match the existing indentation (the surrounding card uses 4-space indent). The card's `<section>`, `<h2>`, label, and closing tags are unchanged.

- [ ] **Step 5: Run tests, verify they pass**

Run: `go test ./... -run "TestSettingsPage" -v`
Expected: all existing settings tests still PASS (FlipsToggleOn, FlipsToggleOff, FlipsOfflineModeOn, FlipsOfflineModeOff, BothTogglesWrittenEveryPOST, AdminCanView, StaffForbidden, RendersOfflineToggle) AND the two new tests PASS (RendersLockedToggleWhenEnvLocked, SkipsOfflineModeWhenEnvLocked).

Run full suite:
Run: `go test ./...`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add handlers_settings.go templates/admin_settings.html handlers_settings_test.go
git commit -m "$(cat <<'EOF'
feat(settings): admin Settings page surfaces the env-lock state

HandleSettings exposes OfflineModeLocked to the template via the new
IsOfflineEnvLocked accessor. Template branches on that flag to render
the toggle with checked+disabled attributes and switches the help
copy to the lock explanation. HandleSettingsPost loop entries gain a
skipWrite field that gates the offline_mode write while locked; the
staff_can_import_patrons write path is unaffected.

Refs cs408-go-stack-85j.
EOF
)"
```

---

## Task 3: Startup log line

Logging-only change. No unit test (main.go is not unit-tested in this codebase); manual verification step at the end of the task.

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Locate the insertion point in `main.go`**

Open `main.go` and find this block (around line 32-34):

```go
dm := NewDatabaseManager(dataDir + "/" + dbName)
dm.SeedDefaultUsers()

// LIBRESHELF_SKIP_SEED skips the book + cover seed steps so the
```

Insert the new log line BETWEEN `dm.SeedDefaultUsers()` and the comment line.

- [ ] **Step 2: Add the log line**

Edit `main.go` so the block reads:

```go
dm := NewDatabaseManager(dataDir + "/" + dbName)
dm.SeedDefaultUsers()

if IsOfflineEnvLocked() {
	log.Printf("LIBRESHELF_OFFLINE=true is locking offline mode; runtime DB setting will be ignored until the env var is unset.")
}

// LIBRESHELF_SKIP_SEED skips the book + cover seed steps so the
```

`log` is already imported by `main.go`.

- [ ] **Step 3: Verify the binary builds**

Run: `go build .`
Expected: clean, produces `libreshelf` binary.

- [ ] **Step 4: Manual smoke test**

Run with the lock set:
```bash
LIBRESHELF_OFFLINE=true go run . 2>&1 | head -15
```

Expected: among the early log lines, you should see:
```
2026/MM/DD HH:MM:SS LIBRESHELF_OFFLINE=true is locking offline mode; runtime DB setting will be ignored until the env var is unset.
```

Stop the server with Ctrl-C once you see the line.

Run without the lock to confirm the line does NOT appear:
```bash
go run . 2>&1 | head -15
```

Expected: no line mentioning `LIBRESHELF_OFFLINE`. Stop with Ctrl-C.

- [ ] **Step 5: Run the test suite to confirm no regressions**

Run: `go test ./...`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add main.go
git commit -m "$(cat <<'EOF'
feat(main): log once at startup when LIBRESHELF_OFFLINE is locking

Operator-facing signal that the env var is overriding any DB row.
Fires once per process lifetime, before the seed-cover backfill (which
has its own offline-mode log line on the same path).

Refs cs408-go-stack-85j.
EOF
)"
```

---

## Task 4: Documentation

Docs-only. No code changes. Updates DEC-033 with a supersession callout, appends DEC-034, rewrites the `LIBRESHELF_OFFLINE` subsection in deployment.md.

**Files:**
- Modify: `DECISIONS.md`
- Modify: `docs/deployment.md`

- [ ] **Step 1: Add the supersession callout to DEC-033**

In `DECISIONS.md`, locate the `## DEC-033: Operator-declared offline mode (env var + admin toggle, no auto-detect)` header. Just below the `**Date:**` line, before the `**Decision:**` line, insert:

```markdown
**Superseded in part by DEC-034 (2026-05-15):** the precedence rule was flipped from "settings row wins over env var" to "env var=true acts as a deployment lock that overrides the DB row." See DEC-034 for current behavior. The rest of this entry (env var name, settings key, why-not-auto-detect, call sites) still applies.

```

(Leave one blank line after the supersession paragraph before `**Decision:**`.)

- [ ] **Step 2: Append DEC-034 at the end of DECISIONS.md**

Append:

```markdown

---

## DEC-034: LIBRESHELF_OFFLINE acts as a deployment lock (precedence flip)

**Date:** 2026-05-15 (cs408-go-stack-85j, branch `feat/offline-mode-env-lock`).

**Decision:** Flip the precedence rule established in DEC-033. `LIBRESHELF_OFFLINE=true` at startup now overrides any `offline_mode` row in the settings table. Setting the env var to any other value (false, 1, yes, unset) leaves the DB row in control. Only the case-insensitive string `"true"` locks. The admin Settings UI renders the offline-mode toggle as checked+disabled with an explanatory note when locked; the handler skips writing the offline_mode row while locked. Other settings (currently `staff_can_import_patrons`) are unaffected by the lock.

**Why the flip:** During manual test of PR #78 (A0), Tim hit the precedence trap that the original DEC-033 design created. The admin Settings handler writes every known setting on every POST (treating absent checkboxes as off). A Save-while-exploring with the offline_mode checkbox unchecked persisted `offline_mode=false` to the DB. On the next restart with `LIBRESHELF_OFFLINE=true`, the DB row silently overrode the env var; OL Lookup returned 200 instead of 503. No log line, no warning. For the documented audience (prisons, secure facilities, air-gapped deployments) the operator needs confidence that an env-var declaration cannot be silently undone by a prior UI Save.

**Asymmetric lock:** Only `=true` locks. There is no "lock online" use case. An operator who deploys in a connected environment simply does not set the env var; the runtime UI is the source of truth.

**Predicate behavior:**
- `offlineEnvDefault=true` (env locks): `IsExternalAllowed` returns false. DB is not consulted. A DB fault during the lock period does not change behavior.
- `offlineEnvDefault=false` (no lock): `IsExternalAllowed` consults the DB row. Empty row defaults to allow (true). DB fault defaults to allow (true) -- fail-open, since there is no env signal to fall back to.

**UI behavior:**
- Locked: toggle rendered as `checked disabled`, help copy reads "Locked by the LIBRESHELF_OFFLINE env var. Clear the env var and restart the server to edit this setting from the UI."
- Unlocked: toggle rendered from the DB row, help copy reads the standard offline-mode description.

**Handler behavior:**
- Locked: `HandleSettingsPost` skips writing the offline_mode row regardless of the submitted form value. The `staff_can_import_patrons` write path is unaffected.
- Unlocked: `HandleSettingsPost` writes every known setting on every POST per the original DEC-033 design (and the cross-toggle non-interference test in PR #78 continues to pin that behavior).

**Startup log:** When locked, `main.go` logs a single line at startup: "LIBRESHELF_OFFLINE=true is locking offline mode; runtime DB setting will be ignored until the env var is unset."

**Related:**
- DEC-033 -- the original A0 precedence decision being superseded in part.
- bd issue `cs408-go-stack-85j` -- the bug report and four-option brainstorm.
- bd issue `cs408-go-stack-ahq` -- parent A0 work that shipped DEC-033 and surfaced this issue.
- Spec: `docs/specs/2026-05-15-offline-mode-env-lock-design.md`.
```

- [ ] **Step 3: Rewrite the LIBRESHELF_OFFLINE subsection in `docs/deployment.md`**

Locate the subsection that currently reads:

```markdown
### `LIBRESHELF_OFFLINE` (optional)

Set to `true` to declare this deployment offline at startup. When on,
LibreShelf does not attempt any outbound HTTP for Open Library lookup,
seed-cover backfill, or future external metadata sources. Admin can also
flip this at runtime via Settings; the runtime setting wins over the env
var. Default: `false`.

Use for deployments where outbound internet access is unavailable or
policy-restricted (prisons, secure facilities, air-gapped networks). To
set via systemd, add an `Environment=` line in `deploy/libreshelf.service`:

```
Environment=LIBRESHELF_OFFLINE=true
```

Then `sudo systemctl daemon-reload && sudo systemctl restart libreshelf`.
```

Replace the entire subsection (header included) with:

```markdown
### `LIBRESHELF_OFFLINE` (optional)

Set to `true` to lock this deployment in offline mode. When set,
LibreShelf does not attempt any outbound HTTP for Open Library lookup,
seed-cover backfill, or future external metadata sources. The admin
Settings UI shows the offline-mode toggle as checked and disabled; it
cannot be edited until the env var is unset and the server is
restarted. Default: unset (no lock).

Only the case-insensitive string `"true"` locks. Any other value
(`false`, `1`, `yes`, etc.) is treated the same as unset.

When the env var is not locking, the admin Settings UI is the source
of truth for offline mode. Admin can flip the toggle at runtime; the
DB row persists across restarts.

Use the env-var lock for deployments where outbound internet access is
unavailable or policy-restricted (prisons, secure facilities,
air-gapped networks). Use the runtime UI toggle for deployments that
need temporary offline mode (network maintenance windows, etc).

To set via systemd, add an `Environment=` line in
`deploy/libreshelf.service`:

```
Environment=LIBRESHELF_OFFLINE=true
```

Then `sudo systemctl daemon-reload && sudo systemctl restart libreshelf`.
```

- [ ] **Step 4: Verify nothing else broke**

Run: `go test ./...`
Expected: all green (docs-only change, but sanity check).

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add DECISIONS.md docs/deployment.md
git commit -m "$(cat <<'EOF'
docs: DEC-034 records the offline-mode precedence flip

DEC-033 gets a supersession callout at the top of its body; DEC-034
appended at the end of the decisions log with the full new behavior
(predicate, UI, handler, startup log). deployment.md LIBRESHELF_OFFLINE
subsection rewritten to describe the lock semantics and the strict
"true"-only parsing.

Refs cs408-go-stack-85j.
EOF
)"
```

---

## Self-Review

**1. Spec coverage:**

| Spec requirement | Implemented in |
|---|---|
| LIBRESHELF_OFFLINE=true locks; other values leave DB in control | Task 1 (predicate) |
| Asymmetric lock (no online lock) | Task 1 |
| Only "true" case-insensitive locks (strict parsing) | Task 1 (preserved from existing init()) |
| New `IsOfflineEnvLocked()` accessor | Task 1 |
| Replace `isExternalAllowedFn` with `isExternalAllowedFromDB` | Task 1 |
| DB-error policy: fail-open when unlocked, lock-wins when locked | Task 1 + tests |
| Admin Settings GET surfaces lock state via OfflineModeLocked | Task 2 |
| HandleSettings offline_mode default changes from env to false | Task 2 |
| HandleSettingsPost skipWrite flag on offline_mode loop entry | Task 2 |
| Template branches on OfflineModeLocked for checked+disabled and copy | Task 2 |
| Cross-toggle test unchanged (env unlocked default) | Task 2 (no change needed) |
| Startup log line in main.go when locked | Task 3 |
| Manual verification of startup log | Task 3 Step 4 |
| DEC-033 supersession callout | Task 4 |
| DEC-034 new entry | Task 4 |
| deployment.md LIBRESHELF_OFFLINE subsection rewrite | Task 4 |
| Withdrawing existing `offlineEnvDefault` UI default in favor of false | Task 2 |
| New tests: env-lock-beats-DB-online, DB-error-when-locked, IsOfflineEnvLocked | Task 1 |
| New tests: RendersLockedToggleWhenEnvLocked, SkipsOfflineModeWhenEnvLocked | Task 2 |

**2. Placeholder scan:** No TBDs, no TODOs, no "similar to above," no "add appropriate error handling." Each step has the exact code or markup to write.

**3. Type consistency:** `IsExternalAllowed`, `isExternalAllowedFromDB`, `IsOfflineEnvLocked`, `offlineEnvDefault`, `OfflineModeLocked` (template), `withOfflineEnvDefault` (test helper) are used consistently across Task 1 (definitions) and Task 2 (consumers). The `setting` struct's `skipWrite` field is defined and used in Task 2 only; no other consumer.

**Test helpers used:** `setupTestDB(t)` (db_test.go:19), `mustCreateUser(t, dm, username, role)` (db_test.go:32), `setupTestRouter(t)`, `loginAs(t, dm, username, role)`. All confirmed present in the codebase from PR #78 verification.

---

## Execution

Plan complete and saved to `docs/plans/2026-05-15-offline-mode-env-lock.md`. Two execution options:

1. **Subagent-Driven (recommended)** -- dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** -- run the tasks in this session using executing-plans with checkpoints.

Which approach?
