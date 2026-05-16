# Offline Mode Declaration (Subproject A0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an operator-declared offline mode that gates all outbound HTTP calls (currently just Open Library; Google Books arrives in subproject A). Source of truth is the existing `settings` table with `LIBRESHELF_OFFLINE` as the startup default. Admin-only toggle in the existing Settings page.

**Architecture:** New `network.go` exposes `IsExternalAllowed(dm) bool`. The function reads the `offline_mode` row from the existing key-value `settings` table; if absent, falls back to the env-var default captured at startup. Wires into the three current external-call entry points: `FetchOpenLibraryBook`, `HandleOpenLibraryLookup`, and `FetchAndStoreSeedCovers`. Admin Settings page gains a checkbox using the same pattern as `staff_can_import_patrons`.

**Tech Stack:** Go 1.25.9+, Gin, html/template, SQLite via modernc.org/sqlite, existing test helpers `setupTestRouter` + `loginAs`.

**Spec:** `docs/specs/2026-05-15-google-books-fallback-design.md` (Subproject A0 section)

---

## File Structure

- **Create** `network.go` -- `IsExternalAllowed(dm *DatabaseManager) bool`, sentinel error `ErrExternalDisabled`, package-level `offlineEnvDefault` populated in `init()`. Plus a testable inner `isExternalAllowedFn(dm, offlineFromEnv)` so tests can avoid env mucking.
- **Create** `network_test.go` -- unit tests for the inner function (env-default-true, env-default-false, settings-override-on, settings-override-off, settings-DB-error fallback).
- **Modify** `openlibrary.go:253` -- `FetchOpenLibraryBook` does not change signature; instead add a new exported entry point `FetchOpenLibraryBookGated(ctx, dm, isbn)` that calls the predicate first and returns `ErrExternalDisabled` if blocked. (Keeping `FetchOpenLibraryBook` un-gated avoids breaking the existing test pattern in `openlibrary_test.go` that calls it directly against `httptest.NewServer`.)
- **Modify** `handlers_books.go:122` -- `HandleOpenLibraryLookup` uses the new gated entry point, returns 503 with `{"error":"offline_mode"}` on `ErrExternalDisabled`.
- **Modify** `db.go:2528` -- `FetchAndStoreSeedCovers` takes `dm` as receiver already; first check `IsExternalAllowed(dm)`, log once, return early if not allowed.
- **Modify** `handlers_settings.go` -- read + write `offline_mode` alongside `staff_can_import_patrons`.
- **Modify** `templates/admin_settings.html` -- add a new section / card with the offline-mode checkbox, copying the form-switch markup of the existing toggle.
- **Modify** `handlers_settings_test.go` -- add tests for the new setting through the page handler.
- **Modify** `docs/deployment.md` -- document `LIBRESHELF_OFFLINE` env var alongside `PORT` / `DATA_DIR`.
- **Modify** `DECISIONS.md` -- new DEC-033 entry.

---

## Task 1: Sentinel error + predicate scaffold

**Files:**
- Create: `network.go`
- Test: `network_test.go`

- [ ] **Step 1: Write failing tests for the inner predicate**

Create `network_test.go`:

```go
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
```

Note: `setupTestDB(t)` (db_test.go:19) and `mustCreateUser(t, dm, ...)` (db_test.go:32) are existing helpers. Both confirmed present.

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./... -run TestIsExternalAllowed -v`
Expected: FAIL with "undefined: isExternalAllowedFn"

- [ ] **Step 3: Write minimal predicate**

Create `network.go`:

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
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./... -run TestIsExternalAllowed -v`
Expected: 4 PASS

- [ ] **Step 5: Commit**

```bash
git add network.go network_test.go
git commit -m "feat(network): IsExternalAllowed predicate + ErrExternalDisabled

Reads offline_mode setting, falls back to LIBRESHELF_OFFLINE env var
captured at startup. Predicate-only -- not yet wired into call sites.

Refs cs408-go-stack-8gj (A0 prerequisite for Google Books fallback)."
```

---

## Task 2: Gate OL chain entry point

**Files:**
- Modify: `openlibrary.go:253` (add `FetchOpenLibraryBookGated` wrapper)
- Test: `openlibrary_test.go` (add gated-path test)

- [ ] **Step 1: Write failing test**

Add to `openlibrary_test.go`:

```go
func TestFetchOpenLibraryBookGated_OfflineReturnsSentinel(t *testing.T) {
	dm := setupTestDB(t)
	adminID := mustCreateUser(t, dm, "admin_off1", "admin")
	if err := dm.SetSetting("offline_mode", "true", adminID); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	_, err := FetchOpenLibraryBookGated(context.Background(), dm, "9780000000000")
	if !errors.Is(err, ErrExternalDisabled) {
		t.Errorf("want ErrExternalDisabled, got %v", err)
	}
}
```

Imports needed: `context`, `errors` (likely already present; add if not).

- [ ] **Step 2: Run test, verify it fails**

Run: `go test ./... -run TestFetchOpenLibraryBookGated_OfflineReturnsSentinel -v`
Expected: FAIL with "undefined: FetchOpenLibraryBookGated"

- [ ] **Step 3: Add the wrapper**

Add to `openlibrary.go` (place just above the existing `FetchOpenLibraryBook` at line 253):

```go
// FetchOpenLibraryBookGated is the offline-aware entry point for the
// admin Lookup path and any future caller that should respect the
// operator's offline-mode declaration. Returns ErrExternalDisabled
// without making any HTTP attempt when external calls are blocked.
//
// Tests that need to drive the OL chain against httptest.NewServer
// should keep calling FetchOpenLibraryBook directly.
func FetchOpenLibraryBookGated(ctx context.Context, dm *DatabaseManager, isbn string) (*OpenLibraryBook, error) {
	if !IsExternalAllowed(dm) {
		return nil, ErrExternalDisabled
	}
	return FetchOpenLibraryBook(ctx, isbn)
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./... -run TestFetchOpenLibraryBookGated -v`
Expected: PASS

Then full suite to make sure nothing else broke:

Run: `go test ./...`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add openlibrary.go openlibrary_test.go
git commit -m "feat(openlibrary): FetchOpenLibraryBookGated wrapper respects offline mode

Returns ErrExternalDisabled without HTTP when offline. Existing
un-gated FetchOpenLibraryBook stays for httptest-driven tests."
```

---

## Task 3: Wire the gated entry point into HandleOpenLibraryLookup

**Files:**
- Modify: `handlers_books.go:122`
- Test: `handlers_books_test.go`

- [ ] **Step 1: Write failing handler test**

Add to `handlers_books_test.go`:

```go
func TestHandleOpenLibraryLookup_OfflineReturns503(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin1", "admin")
	adminID := mustCreateUser(t, dm, "admin_lookup_off", "admin")
	if err := dm.SetSetting("offline_mode", "true", adminID); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/openlibrary/isbn/9780000000000", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "offline_mode") {
		t.Errorf("body should mention offline_mode, got %q", rr.Body.String())
	}
}
```

Imports needed: `net/http`, `net/http/httptest`, `strings`, `testing` (most already present; add as needed).

- [ ] **Step 2: Run test, verify it fails**

Run: `go test ./... -run TestHandleOpenLibraryLookup_OfflineReturns503 -v`
Expected: FAIL. Without the wiring the handler will either return 502 (upstream error against the unreachable real OL) or 400 -- either way not 503.

- [ ] **Step 3: Update the handler**

Edit `handlers_books.go:122` -- replace the current `HandleOpenLibraryLookup` body. New version:

```go
func HandleOpenLibraryLookup(c *gin.Context) {
	cleaned := stripISBNFormatting(c.Param("isbn"))
	if !IsValidISBN(cleaned) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_isbn"})
		return
	}

	dm := getDB(c)
	book, err := FetchOpenLibraryBookGated(c.Request.Context(), dm, cleaned)
	if errors.Is(err, ErrExternalDisabled) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "offline_mode"})
		return
	}
	if errors.Is(err, ErrOpenLibraryNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("HandleOpenLibraryLookup: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream_unavailable"})
		return
	}

	if book.Description != "" {
		book.DescriptionSource = "openlibrary"
	}

	c.JSON(http.StatusOK, book)
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./... -run TestHandleOpenLibraryLookup -v`
Expected: existing OL lookup tests still PASS, new offline test PASSES.

Full suite:

Run: `go test ./...`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add handlers_books.go handlers_books_test.go
git commit -m "feat(handlers): admin OL Lookup returns 503 in offline mode

HandleOpenLibraryLookup now calls FetchOpenLibraryBookGated. Returns
{\"error\":\"offline_mode\"} with HTTP 503 when external calls are
blocked, so the admin Lookup JS can render a clear banner."
```

---

## Task 4: Gate the seed-cover backfill

**Files:**
- Modify: `db.go:2528` (early-return when offline)
- Test: `db_test.go` (or wherever FetchAndStoreSeedCovers tests live; verify first)

- [ ] **Step 1: Locate existing test surface for FetchAndStoreSeedCovers**

Run: `grep -n "FetchAndStoreSeedCovers" db_test.go covers_test.go handlers_coverage_test.go`
If a test file already exercises this function, add the new test alongside; otherwise add to `db_test.go`.

- [ ] **Step 2: Write failing test**

```go
func TestFetchAndStoreSeedCovers_OfflineSkipsWithoutHTTP(t *testing.T) {
	dm := setupTestDB(t)
	adminID := mustCreateUser(t, dm, "admin_seed_off", "admin")
	if err := dm.SetSetting("offline_mode", "true", adminID); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	// Insert a book with ISBN but no cover so the function has work to
	// do if it ignored offline mode.
	_, err := dm.db.Exec(`INSERT INTO books (title, isbn) VALUES (?, ?)`,
		"Offline Test Book", "9780000000000")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Hit the function with a context that would expire instantly if any
	// HTTP attempt slipped through; the offline short-circuit must fire
	// before the request loop.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	dm.FetchAndStoreSeedCovers(ctx)

	// If we got here without hanging or panicking, the function returned
	// quickly. Verify the book still has no cover (the function did not
	// reach the save step).
	var coverFilename *string
	if err := dm.db.QueryRow(
		`SELECT cover_filename FROM books WHERE isbn = ?`,
		"9780000000000",
	).Scan(&coverFilename); err != nil {
		t.Fatalf("select cover: %v", err)
	}
	if coverFilename != nil {
		t.Errorf("offline mode allowed a cover to be written: %q", *coverFilename)
	}
}
```

Imports: `context`, `time` (likely already present).

- [ ] **Step 3: Run test, verify it fails**

Run: `go test ./... -run TestFetchAndStoreSeedCovers_OfflineSkipsWithoutHTTP -v`
Expected: timeout or HTTP error visible in log output (the function tries to hit real OL since offline mode is not yet wired in).

- [ ] **Step 4: Add the gate**

Edit `db.go:2528`. Insert at the top of `FetchAndStoreSeedCovers`, before the `dm.db.Query`:

```go
func (dm *DatabaseManager) FetchAndStoreSeedCovers(ctx context.Context) {
	if !IsExternalAllowed(dm) {
		log.Printf("FetchAndStoreSeedCovers: offline mode -- skipping seed cover backfill")
		return
	}

	rows, err := dm.db.Query(`
		SELECT id, isbn FROM books
		WHERE cover_filename IS NULL AND isbn IS NOT NULL AND isbn != ''`)
	// ... rest unchanged ...
```

- [ ] **Step 5: Run tests, verify they pass**

Run: `go test ./... -run TestFetchAndStoreSeedCovers -v`
Expected: PASS (new offline test green; any existing tests still green).

Full suite:

Run: `go test ./...`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add db.go db_test.go
git commit -m "feat(seed): FetchAndStoreSeedCovers short-circuits in offline mode

When offline_mode is on, the seed backfill logs once and returns
without attempting any HTTP. Prevents the startup loop from burning
60s of context timeout against blocked outbound."
```

---

## Task 5: Admin Settings UI + handler for offline_mode toggle

**Files:**
- Modify: `handlers_settings.go`
- Modify: `templates/admin_settings.html`
- Modify: `handlers_settings_test.go`

- [ ] **Step 1: Write failing tests for the new toggle**

Add to `handlers_settings_test.go`:

```go
func TestSettingsPagePOST_FlipsOfflineModeOn(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin1", "admin")

	if dm.GetSettingBool("offline_mode", false) {
		t.Fatalf("offline_mode should default off")
	}

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("offline_mode", "on")
	// Preserve the other toggle's state by sending it absent; the handler
	// rewrites both toggles per POST, which is the existing pattern.
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if !dm.GetSettingBool("offline_mode", false) {
		t.Errorf("offline_mode should be true after POST with checkbox=on")
	}
}

func TestSettingsPagePOST_FlipsOfflineModeOff(t *testing.T) {
	router, dm := setupTestRouter(t)
	adminID := mustCreateUser(t, dm, "admin_off_init", "admin")
	_ = dm.SetSetting("offline_mode", "true", adminID)
	sess, csrf := loginAs(t, dm, "admin1", "admin")

	form := url.Values{}
	form.Set("csrf_token", csrf)
	// offline_mode absent = off
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if dm.GetSettingBool("offline_mode", true) {
		t.Errorf("offline_mode should be false after POST with checkbox absent")
	}
}

func TestSettingsPageGET_RendersOfflineToggle(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin1", "admin")

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "offline_mode") {
		t.Errorf("expected offline_mode toggle in body, got %q", rr.Body.String())
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./... -run "TestSettingsPagePOST_FlipsOfflineMode|TestSettingsPageGET_RendersOfflineToggle" -v`
Expected: FAIL -- POST tests fail because the handler does not read `offline_mode`; GET test fails because template does not contain the string.

- [ ] **Step 3: Update the handler**

Edit `handlers_settings.go`. Replace the body of both handlers:

```go
func HandleSettings(c *gin.Context) {
	dm := getDB(c)
	renderTemplate(c, "admin_settings", gin.H{
		"Title":   "Settings",
		"Error":   readAndClearFlash(c, flashKindError),
		"Success": readAndClearFlash(c, flashKindSuccess),
		"Settings": gin.H{
			"StaffCanImportPatrons": dm.GetSettingBool("staff_can_import_patrons", false),
			"OfflineMode":           dm.GetSettingBool("offline_mode", offlineEnvDefault),
		},
	})
}

func HandleSettingsPost(c *gin.Context) {
	user := c.MustGet("user").(*User)
	dm := getDB(c)

	type setting struct {
		key   string
		field string
	}
	for _, s := range []setting{
		{"staff_can_import_patrons", "staff_can_import_patrons"},
		{"offline_mode", "offline_mode"},
	} {
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

- [ ] **Step 4: Update the template**

Edit `templates/admin_settings.html`. Add a new card between the existing "Patron management" card and the submit button:

```html
    <section class="card mb-3">
        <div class="card-body">
            <h2 class="h6 mb-3">External API access</h2>

            <div class="form-check form-switch">
                <input class="form-check-input" type="checkbox" role="switch"
                       id="offline_mode" name="offline_mode"
                       {{if .Settings.OfflineMode}}checked{{end}}>
                <label class="form-check-label" for="offline_mode">
                    <strong>Offline mode (disable external API calls)</strong>
                </label>
                <div class="form-text small">
                    When enabled, LibreShelf will not contact Open Library
                    (or future external metadata sources) for cover art
                    or book information. Book entry continues to work
                    using whatever information staff type in manually.
                    Use this for deployments without internet access or
                    where outbound HTTP is policy-restricted.
                </div>
            </div>
        </div>
    </section>
```

- [ ] **Step 5: Run tests, verify they pass**

Run: `go test ./... -run "TestSettingsPage" -v`
Expected: existing settings tests still PASS, new offline tests PASS.

Full suite:

Run: `go test ./...`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add handlers_settings.go templates/admin_settings.html handlers_settings_test.go
git commit -m "feat(settings): admin toggle for offline_mode

Admin Settings page gains an 'External API access' card with an
offline_mode form-switch. Handler now writes both staff_can_import_patrons
and offline_mode on each POST. GetSettingBool defaults the toggle UI
to the LIBRESHELF_OFFLINE env-var capture, so an offline-by-default
deployment shows the box checked on first visit."
```

---

## Task 6: Deployment docs + DECISIONS.md

**Files:**
- Modify: `docs/deployment.md`
- Modify: `DECISIONS.md`

- [ ] **Step 1: Document LIBRESHELF_OFFLINE in deployment.md**

Locate the env-vars section in `docs/deployment.md` (look for `PORT`, `DATA_DIR`, etc.). Add an entry:

```markdown
### `LIBRESHELF_OFFLINE` (optional)

Set to `true` to declare this deployment offline at startup. When on,
LibreShelf does not attempt any outbound HTTP for Open Library lookup,
seed-cover backfill, or future external metadata sources. Admin can also
flip this at runtime via Settings; the runtime setting wins over the env
var. Default: `false`.

Use for deployments where outbound internet access is unavailable or
policy-restricted (prisons, secure facilities, air-gapped networks).
```

- [ ] **Step 2: Add DEC-033 to DECISIONS.md**

Append a new entry at the end of the decisions log:

```markdown
## DEC-033: Operator-declared offline mode (env var + admin toggle, no auto-detect)

**Decision:** External HTTP calls (Open Library, future Google Books,
future Internet Archive) are gated by an operator-declared offline-mode
predicate. Sources: `LIBRESHELF_OFFLINE` env var as startup default,
`offline_mode` row in the existing `settings` table as runtime override.
The settings row wins when present. Admin-only toggle on the existing
Settings page.

**Why not auto-detect:** Restricted networks are inconsistent. Some allow
outbound HTTPS to some hosts and block others; transparent proxies can
return success codes for blocked URLs; air-gapped facilities have no
reliable probe target. The operator knows the deployment's network
policy. Asking the app to guess from inside the network creates false
positives and negatives that are worse than an explicit declaration.

**Call sites gated by `IsExternalAllowed`:**
- `FetchOpenLibraryBookGated` (admin OL Lookup path)
- `FetchAndStoreSeedCovers` (startup seed backfill)

The un-gated `FetchOpenLibraryBook` remains exported so the existing
httptest-driven tests in `openlibrary_test.go` keep working without
needing a DB.

**Future use:** Subproject A (Google Books) reads the same predicate
before any GB HTTP attempt. Same pattern applies to Internet Archive,
Wikidata, and any future external source -- one gate, one toggle.

**Related:** spec at `docs/specs/2026-05-15-google-books-fallback-design.md`;
bd issues cs408-go-stack-8gj (next subproject) and cs408-go-stack-0eh
(offline workflow context).
```

- [ ] **Step 3: Verify nothing else broke**

Run: `go test ./...`
Expected: all green.

Run: `go vet ./...`
Expected: clean.

Run: `gofmt -l .`
Expected: empty output (no unformatted files).

- [ ] **Step 4: Commit**

```bash
git add docs/deployment.md DECISIONS.md
git commit -m "docs: document offline mode + DEC-033

LIBRESHELF_OFFLINE env var and runtime admin toggle. Decision record
explains why operator-declared beats auto-detect for the prison /
secure-facility audience."
```

---

## Self-Review

**Spec coverage:**
- A0 decision: env var + settings table override -> Tasks 1, 5 (handler reads both, with env-default fallback at the UI level too).
- Admin-only toggle -> Task 5 (route already in admin group; existing forbidden-for-staff test in `handlers_settings_test.go` covers regression).
- `IsExternalAllowed` predicate API -> Task 1.
- Wired into FetchOpenLibraryBook entry point -> Task 2 (wrapper) + Task 3 (handler).
- Wired into FetchAndStoreSeedCovers -> Task 4.
- Tests listed in A0 section of spec -> Tasks 1, 2, 3, 4, 5 (all five test categories covered).
- Docs: deployment.md + DEC-033 -> Task 6.

**Placeholder scan:** No "TBD", "implement appropriate", "similar to above" anywhere. Each task contains the code or markup to write.

**Type consistency:** `IsExternalAllowed` / `isExternalAllowedFn` / `ErrExternalDisabled` / `FetchOpenLibraryBookGated` / `offlineEnvDefault` -- consistent across all tasks. `offline_mode` settings key consistent across handler, UI, predicate, and tests.

**Test helpers used:** `setupTestDB(t)` at db_test.go:19 and `mustCreateUser(t, dm, username, role)` at db_test.go:32. Both confirmed present during plan-writing -- no extraction needed.

---

## Execution

Plan complete and saved to `docs/plans/2026-05-15-offline-mode.md`. Two execution options:

1. **Subagent-Driven (recommended)** -- dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** -- run the tasks in this session using executing-plans with checkpoints.

Which approach?

