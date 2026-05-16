# SaveCoverFromURL Offline-Mode Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Gate `SaveCoverFromURL` with the offline-mode predicate so a stale tab or replayed POST cannot fetch external URLs while the deployment is locked. Skip silently with a success-with-caveat flash banner so the admin knows the cover was not downloaded.

**Architecture:** Mirror A0's `FetchOpenLibraryBookGated` pattern. The un-gated `SaveCoverFromURL(url)` stays for existing httptest-driven tests; a new `SaveCoverFromURLGated(dm, url)` wraps it with the predicate check, returning `ErrExternalDisabled` (the existing A0 sentinel) when blocked. Both handlers (BookCreate, BookUpdate) switch to the gated wrapper, accumulate a `coverSkippedOffline` flag, and emit one of two new flash codes on the success path.

**Tech Stack:** Go 1.25.9+, Gin, html/template, SQLite via modernc.org/sqlite, existing test helpers `setupTestRouter`, `loginAs`, `setupTestDB`, `mustCreateUser`, `withOfflineEnvDefault`, `postBookMultipart`, `validBookFields`, `flashCode`.

**Spec:** `docs/specs/2026-05-15-savecoverfromurl-gate-design.md`

---

## File Structure

- **Modify** `covers.go` -- add `SaveCoverFromURLGated(dm, url)` wrapper just above the existing `SaveCoverFromURL` declaration. Existing un-gated function unchanged.
- **Modify** `covers_test.go` -- add `TestSaveCoverFromURLGated_OfflineReturnsSentinel`. Existing 5 tests on the un-gated function unchanged.
- **Modify** `flash.go` -- add two new entries to the `flashMessages` map.
- **Modify** `handlers_books.go` -- two call-site changes (HandleBookCreate around line 273, HandleBookUpdate around line 499) plus two flash-code branches at the success points (around line 301 for create, line 539 for update).
- **Modify** `handlers_books_test.go` -- add two new handler tests for the offline-skip path. Existing book-create/update tests unchanged.

---

## Task 1: Gated wrapper + unit test

**Files:**
- Modify: `covers.go`
- Modify: `covers_test.go`

- [ ] **Step 1: Write failing test**

Append to `covers_test.go`:

```go
func TestSaveCoverFromURLGated_OfflineReturnsSentinel(t *testing.T) {
	dm := setupTestDB(t)
	withOfflineEnvDefault(t, true)

	// The URL is intentionally .invalid (RFC 6761 reserved TLD) so any
	// accidental HTTP attempt fails fast and obviously. The gate must
	// fire before any HTTP attempt is made.
	_, err := SaveCoverFromURLGated(dm, "https://example.invalid/cover.jpg")
	if !errors.Is(err, ErrExternalDisabled) {
		t.Errorf("want ErrExternalDisabled, got %v", err)
	}
}
```

Imports needed: `errors` (verify it's already in `covers_test.go` imports; add if not). `setupTestDB` is at `db_test.go:19`, `withOfflineEnvDefault` is at `network_test.go:14`, `ErrExternalDisabled` is at `network.go:14`. All in the same `main` package; no import additions needed for those.

- [ ] **Step 2: Run test, verify it fails**

Run: `go test ./... -run TestSaveCoverFromURLGated_OfflineReturnsSentinel -v`
Expected: FAIL with "undefined: SaveCoverFromURLGated"

- [ ] **Step 3: Add the wrapper**

Edit `covers.go`. Place the new function directly above the existing `SaveCoverFromURL` declaration (currently at covers.go:99):

```go
// SaveCoverFromURLGated is the offline-aware entry point for any caller
// that has a DatabaseManager. Returns ErrExternalDisabled without making
// any HTTP attempt when external calls are blocked.
//
// Tests that need to drive the HTTP path against httptest.NewServer
// should keep calling SaveCoverFromURL directly.
func SaveCoverFromURLGated(dm *DatabaseManager, url string) (string, error) {
	if !IsExternalAllowed(dm) {
		return "", ErrExternalDisabled
	}
	return SaveCoverFromURL(url)
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./... -run TestSaveCoverFromURLGated -v`
Expected: PASS.

Run: `go test ./... -run "TestSaveCoverFromURL" -v`
Expected: all 6 PASS (existing 5 + new 1).

Full suite:

Run: `go test ./...`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add covers.go covers_test.go
git commit -m "$(cat <<'EOF'
feat(covers): SaveCoverFromURLGated wrapper respects offline mode

Returns ErrExternalDisabled without HTTP when offline. Existing
un-gated SaveCoverFromURL stays for the existing httptest-driven
tests (TestSaveCoverFromURL_Happy etc).

Refs cs408-go-stack-di7.
EOF
)"
```

---

## Task 2: Wire handlers + flash codes + handler tests

Single TDD cycle because the two handler changes share a flash-code dependency (both new codes must exist before either handler test passes).

**Files:**
- Modify: `flash.go`
- Modify: `handlers_books.go`
- Modify: `handlers_books_test.go`

- [ ] **Step 1: Write the two failing handler tests**

Append to `handlers_books_test.go`:

```go
func TestHandleBookCreate_CoverURLSkippedWhenOffline(t *testing.T) {
	router, dm := setupTestRouter(t)
	withOfflineEnvDefault(t, true)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	fields := validBookFields()
	fields["isbn"] = "9780000000001"
	// .invalid is RFC 6761 reserved TLD; any accidental HTTP attempt
	// will fail fast. The offline gate must fire before any attempt.
	fields["cover_url"] = "https://example.invalid/cover.jpg"

	rr := postBookMultipart(t, router, "/books", sess, csrf, fields, "", nil)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d. body: %s", rr.Code, rr.Body.String())
	}
	if got := flashCode(rr, "flash_success"); got != "book_created_cover_skipped_offline" {
		t.Errorf("expected flash_success=book_created_cover_skipped_offline, got %q", got)
	}

	// The book row should exist with no cover.
	got, err := dm.GetBookByISBN("9780000000001")
	if err != nil {
		t.Fatalf("GetBookByISBN: %v", err)
	}
	if got.CoverFilename != nil {
		t.Errorf("expected nil cover_filename after offline skip, got %q", *got.CoverFilename)
	}
}

func TestHandleBookUpdate_CoverURLSkippedWhenOffline(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	// Pre-seed a book with no cover.
	id, err := dm.CreateBook(&Book{Title: "Existing", QuantityTotal: 1, QuantityAvailable: 1}, []string{"Author"})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}

	// Lock AFTER the seed so seed-time external calls are not relevant
	// (the seed-cover backfill is gated separately and would no-op here
	// anyway since the row has no ISBN).
	withOfflineEnvDefault(t, true)

	fields := map[string]string{
		"title":     "Existing",
		"authors":   "Author",
		"quantity":  "1",
		"cover_url": "https://example.invalid/cover.jpg",
	}
	rr := postBookMultipart(t, router, fmt.Sprintf("/books/%d/edit", id), sess, csrf, fields, "", nil)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d. body: %s", rr.Code, rr.Body.String())
	}
	if got := flashCode(rr, "flash_success"); got != "book_updated_cover_skipped_offline" {
		t.Errorf("expected flash_success=book_updated_cover_skipped_offline, got %q", got)
	}

	// The book row should still have no cover (offline gate skipped the download).
	got, err := dm.GetBookByID(id)
	if err != nil {
		t.Fatalf("GetBookByID: %v", err)
	}
	if got.CoverFilename != nil {
		t.Errorf("expected nil cover_filename after offline skip, got %q", *got.CoverFilename)
	}
}
```

Imports needed: `fmt` (used in `Sprintf("/books/%d/edit", id)`). Verify it's already imported (it is in many test files; double-check `handlers_books_test.go` and add if missing).

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./... -run "TestHandleBookCreate_CoverURLSkippedWhenOffline|TestHandleBookUpdate_CoverURLSkippedWhenOffline" -v`
Expected: both FAIL. The handlers do not yet branch on the offline gate. Failure mode will be a generic flash code mismatch and a real HTTP attempt against example.invalid (which fails with DNS or connection error, depending on the resolver), which produces a different render than the test expects.

- [ ] **Step 3: Add the two new flash codes**

Edit `flash.go`. Locate the `flashMessages` map (currently around lines 35-64). Add two new entries (alphabetic order, or near the existing `book_created` / `book_updated` codes -- match existing style):

```go
	"book_created_cover_skipped_offline": "Book created. Cover URL was skipped because offline mode is on.",
	"book_updated_cover_skipped_offline": "Book updated. Cover URL was skipped because offline mode is on.",
```

- [ ] **Step 4: Update `HandleBookCreate`**

Edit `handlers_books.go` at the `else if coverURL != ""` block (currently around lines 273-281). Before that block, declare the skip flag. The current code is:

```go
	} else if coverURL != "" {
		saved, err := SaveCoverFromURL(coverURL)
		if err != nil {
			log.Printf("HandleBookCreate: SaveCoverFromURL(%q): %v", coverURL, err)
			renderBookCreateForm(c, book, authorsText, "Could not download cover from Open Library. Try uploading a file instead.")
			return
		}
		book.CoverFilename = &saved
	}
```

Replace with (and add the `coverSkippedOffline` declaration just above the `if fh, err := c.FormFile("cover"); err == nil {` block at line 256):

```go
	coverSkippedOffline := false
	if fh, err := c.FormFile("cover"); err == nil {
		// ... existing file-upload branch unchanged ...
	} else if coverURL != "" {
		saved, err := SaveCoverFromURLGated(dm, coverURL)
		switch {
		case errors.Is(err, ErrExternalDisabled):
			log.Printf("HandleBookCreate: cover URL skipped due to offline mode (url=%q)", coverURL)
			coverSkippedOffline = true
		case err != nil:
			log.Printf("HandleBookCreate: SaveCoverFromURLGated(%q): %v", coverURL, err)
			renderBookCreateForm(c, book, authorsText, "Could not download cover from Open Library. Try uploading a file instead.")
			return
		default:
			book.CoverFilename = &saved
		}
	}
```

(The `// ... existing file-upload branch unchanged ...` is shorthand for "leave the original block as it is." Do not actually replace those lines with a comment. The block currently is lines 256-272.)

Then at the success path (currently `setFlash(c, flashKindSuccess, "book_created")` around line 301), branch on the flag:

```go
	if coverSkippedOffline {
		setFlash(c, flashKindSuccess, "book_created_cover_skipped_offline")
	} else {
		setFlash(c, flashKindSuccess, "book_created")
	}
	setFlashDetail(c, book.Title)
```

The redirect line (`c.Redirect(http.StatusFound, ...)`) stays unchanged.

- [ ] **Step 5: Update `HandleBookUpdate`**

Same shape, applied to the BookUpdate handler. The current `else if coverURL != ""` block (lines 499-510) becomes:

```go
	coverSkippedOffline := false
	if fh, err := c.FormFile("cover"); err == nil {
		// ... existing file-upload branch unchanged ...
	} else if coverURL != "" {
		saved, err := SaveCoverFromURLGated(dm, coverURL)
		switch {
		case errors.Is(err, ErrExternalDisabled):
			log.Printf("HandleBookUpdate: cover URL skipped due to offline mode (url=%q)", coverURL)
			coverSkippedOffline = true
		case err != nil:
			log.Printf("HandleBookUpdate: SaveCoverFromURLGated(%q): %v", coverURL, err)
			renderBookEditForm(c, id, book, authorsText, "Could not download cover from Open Library. Try uploading a file instead.")
			return
		default:
			if existing.CoverFilename != nil {
				oldCoverToDelete = *existing.CoverFilename
			}
			book.CoverFilename = &saved
		}
	}
```

Declare `coverSkippedOffline := false` just above the cover-routing block (above the `var oldCoverToDelete string` line at around 477, or right after it -- either works as long as it's in scope at the success path).

At the success path (`setFlash(c, flashKindSuccess, "book_updated")` around line 539), branch:

```go
	if coverSkippedOffline {
		setFlash(c, flashKindSuccess, "book_updated_cover_skipped_offline")
	} else {
		setFlash(c, flashKindSuccess, "book_updated")
	}
	setFlashDetail(c, book.Title)
```

The redirect line stays unchanged.

- [ ] **Step 6: Run tests, verify they pass**

Run: `go test ./... -run "TestHandleBookCreate_CoverURLSkippedWhenOffline|TestHandleBookUpdate_CoverURLSkippedWhenOffline" -v`
Expected: both PASS.

Run: `go test ./... -run "TestBookCreate|TestBookUpdate|TestHandleBookCreate|TestHandleBookUpdate" -v`
Expected: all existing book-create/update tests still PASS plus the two new ones.

Full suite:

Run: `go test ./...`
Expected: all green.

- [ ] **Step 7: Commit**

```bash
git add flash.go handlers_books.go handlers_books_test.go
git commit -m "$(cat <<'EOF'
feat(handlers): book create+update skip cover_url when offline

HandleBookCreate and HandleBookUpdate now call SaveCoverFromURLGated.
On ErrExternalDisabled the handler logs a server-side line, sets a
local coverSkippedOffline flag, and proceeds with the create/update.
Two new flash codes (book_created_cover_skipped_offline and
book_updated_cover_skipped_offline) surface a success-with-caveat
banner so the admin knows the cover was not downloaded.

Closes the third external-HTTP entry point's offline-mode gate, the
last item flagged in the A0 final code review (cs408-go-stack-di7).
With this in, the call-site audit for DEC-033 + DEC-034 is complete.

Refs cs408-go-stack-di7.
EOF
)"
```

---

## Self-Review

**1. Spec coverage:**

| Spec requirement | Implemented in |
|---|---|
| `SaveCoverFromURLGated` wrapper, mirrors FetchOpenLibraryBookGated | Task 1 |
| Reuse `ErrExternalDisabled` sentinel | Task 1 (returned by wrapper) and Task 2 (matched by handler) |
| Un-gated `SaveCoverFromURL` preserved | Task 1 (not modified) |
| Handler-level skip + success-with-caveat banner | Task 2 |
| Server-side log line on skip path | Task 2 (Steps 4 + 5) |
| Two new flash codes | Task 2 Step 3 |
| `TestSaveCoverFromURLGated_OfflineReturnsSentinel` unit test | Task 1 |
| `TestHandleBookCreate_CoverURLSkippedWhenOffline` | Task 2 |
| `TestHandleBookUpdate_CoverURLSkippedWhenOffline` | Task 2 |
| Existing tests unchanged | Both tasks (verified in test runs) |
| No DECISIONS.md / deployment.md / template changes | Confirmed in plan; this is pure implementation completing DEC-033+DEC-034 |

**2. Placeholder scan:** No TBDs, no TODOs. The "// ... existing file-upload branch unchanged ..." marker in Task 2 Steps 4-5 is shorthand and explicitly explained in-line so the engineer does not literally paste the comment. Each code block contains the actual content to write.

**3. Type consistency:** `SaveCoverFromURLGated`, `ErrExternalDisabled`, `IsExternalAllowed`, `coverSkippedOffline` (local bool), `book_created_cover_skipped_offline` and `book_updated_cover_skipped_offline` (flash codes) all used consistently across both tasks. `withOfflineEnvDefault` helper, `setupTestDB`, `loginAs`, `postBookMultipart`, `validBookFields`, `flashCode`, `dm.CreateBook`, `dm.GetBookByID`, `dm.GetBookByISBN` are all existing project helpers used as-is.

---

## Execution

Plan complete and saved to `docs/plans/2026-05-15-savecoverfromurl-gate.md`. Two execution options:

1. **Subagent-Driven (recommended)** -- dispatch a fresh subagent per task, review between tasks.
2. **Inline Execution** -- run the tasks in this session using executing-plans with checkpoints.

Which approach?
