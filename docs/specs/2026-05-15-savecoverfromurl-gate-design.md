# Gate SaveCoverFromURL with the Offline-Mode Predicate

Status: Design approved 2026-05-15. Implementation plan to follow.
Owner: Tim Palacios
Related bd issue: cs408-go-stack-di7
Completes call-site audit for: DEC-033, DEC-034.

## Context

A0 (PR #78) introduced `IsExternalAllowed` and gated two of the three
known external-HTTP entry points: `FetchOpenLibraryBookGated` and
`FetchAndStoreSeedCovers`. The third entry point, `SaveCoverFromURL`,
was not gated and was missed during the A0 audit.

`SaveCoverFromURL` is called from `HandleBookCreate` (handlers_books.go:274)
and `HandleBookUpdate` (handlers_books.go:500) when the book form submits
a non-empty `cover_url` hidden field. The JS that populates that field
runs only after a successful OL Lookup, which is now gated and returns
503 in offline mode, so under normal flow the `cover_url` stays empty.
But three paths still hit the gap:

1. Stale tab: admin opened the form while online, then offline mode
   was set on the server, then admin submitted.
2. Manual form replay: admin used browser back + resubmit from history,
   posting a stale `cover_url`.
3. Crafted POST: any authenticated admin can fabricate a POST with an
   arbitrary URL in `cover_url`.

For the operator audience the offline-mode contract claims "no external
HTTP calls" while locked, and this path violates that claim. The
exposure is narrow but real, and the fix is small.

DEC-034 already specifies the pattern: "Same pattern applies to Internet
Archive, Wikidata, and any future external source -- one gate, one
toggle." This spec just completes the existing call-site audit; it does
not introduce a new precedence decision or modify the predicate's
contract.

## Goals

- `SaveCoverFromURL` cannot fetch any URL when offline mode is on
  (either via env-var lock or DB-row setting).
- Stale-tab and form-replay paths gracefully save the book without
  the cover, with an explanatory flash banner so the admin knows what
  happened.
- Existing tests against the un-gated `SaveCoverFromURL` (which use
  `httptest.NewServer`) continue to work unchanged.

## Non-goals

- SSRF mitigation for arbitrary URLs submitted by admins or crafted
  POSTs (e.g. localhost probes, cloud-metadata endpoint). Pre-existing
  concern, out of scope here. Can be filed separately as a P3 if Tim
  wants tightening.
- New JS-side handling of OL Lookup 503 responses. Out of scope; that
  flow already works (OL Lookup returns 503, JS does not populate
  `cover_url`). If the JS banner is unclear in the offline case, file
  a separate bd issue.
- New flash kind (e.g. `flashKindWarning`). The existing two-kind
  system handles this case via a new flash code mapped to a
  longer-form success message.
- DECISIONS.md entry. This is an implementation completing existing
  designs (DEC-033 + DEC-034), not a new design decision.

## Design decisions captured

| Decision | Value | Rationale |
|----------|-------|-----------|
| Defense layer | Both: handler-level gate + internal SaveCoverFromURL gate | Mirrors A0's `FetchOpenLibraryBookGated` wrapping `FetchOpenLibraryBook` pattern |
| Behavior when offline + cover_url submitted | Strip silently + success banner with caveat | Book entry continues to work; admin knows what was skipped |
| Banner mechanism | New flash code + existing flashKindSuccess | Avoids introducing a third flash kind; SuccessDetail still carries the book title |
| Sentinel error | Reuse existing `ErrExternalDisabled` | One sentinel across all external-HTTP gates |
| Server-side logging | `log.Printf` line on the skip path | Operational signal for unexpected flows (stale tab, replay) |

## Architecture

### `covers.go` -- new gated wrapper

```go
// SaveCoverFromURLGated is the offline-aware entry point for any
// caller that has a DatabaseManager. Returns ErrExternalDisabled
// without making any HTTP attempt when external calls are blocked.
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

The un-gated `SaveCoverFromURL` stays for the existing test suite in
`covers_test.go` (TestSaveCoverFromURL_Happy, _HTTPError, _TooLarge,
_BadMime, _PNGContentType). Those tests should not need a DM.

### `handlers_books.go` -- two call-site changes

Both handlers (`HandleBookCreate` around line 273 and `HandleBookUpdate`
around line 499) follow the same shape:

```go
} else if coverURL != "" {
    saved, err := SaveCoverFromURLGated(dm, coverURL)
    switch {
    case errors.Is(err, ErrExternalDisabled):
        log.Printf("HandleBookCreate: cover URL skipped due to offline mode (url=%q)", coverURL)
        coverSkippedOffline = true
        // book.CoverFilename stays nil; flash gets set after successful create.
    case err != nil:
        log.Printf("HandleBookCreate: SaveCoverFromURLGated(%q): %v", coverURL, err)
        renderBookCreateForm(c, book, authorsText, "Could not download cover from Open Library. Try uploading a file instead.")
        return
    default:
        book.CoverFilename = &saved
    }
}
```

(The log line uses `HandleBookCreate` for the create handler and
`HandleBookUpdate` for the update handler. Otherwise identical.)

A local `coverSkippedOffline bool` accumulates the skip flag and gets
consulted at the success branch to choose the flash code:

```go
// In HandleBookCreate, replacing the existing setFlash(success, "book_created"):
if coverSkippedOffline {
    setFlash(c, flashKindSuccess, "book_created_cover_skipped_offline")
} else {
    setFlash(c, flashKindSuccess, "book_created")
}
setFlashDetail(c, book.Title)
```

Same shape at the BookUpdate success path with `book_updated*` codes.

### `flash.go` -- two new message codes

Add to the `flashMessages` map (currently at flash.go:35-64):

```go
"book_created_cover_skipped_offline": "Book created. Cover URL was skipped because offline mode is on.",
"book_updated_cover_skipped_offline": "Book updated. Cover URL was skipped because offline mode is on.",
```

The catalog renders Success + SuccessDetail concatenated by the
existing template logic, so the user sees:
"Book created. Cover URL was skipped because offline mode is on.
*The Title*"

## Tests

### New handler tests in `handlers_books_test.go`

`TestHandleBookCreate_CoverURLSkippedWhenOffline`:
- `setupTestRouter(t)`, `withOfflineEnvDefault(t, true)`,
  `loginAs(t, dm, "admin1", "admin")`.
- POST `/books` with: csrf token, title="Offline Test", authors="Test
  Author", isbn="9780000000001", cover_url="https://example.invalid/cover.jpg",
  no file upload.
- Assert: 303 redirect (book was created), DB has the row, the row's
  `cover_filename` is NULL, the `flash_success` cookie value is
  `"book_created_cover_skipped_offline"`.
- The URL is intentionally `.invalid` (RFC 6761 reserved TLD) so any
  accidental HTTP attempt fails fast and obviously; the gate must fire
  before that anyway.

`TestHandleBookUpdate_CoverURLSkippedWhenOffline`:
- Same setup, but pre-seed a book row first via direct INSERT.
- POST `/books/<id>/edit` with cover_url submitted and offline mode
  locked.
- Assert: 303 redirect, DB row's cover_filename is unchanged (or NULL
  if pre-seeded that way), the `flash_success` cookie value is
  `"book_updated_cover_skipped_offline"`.

### New unit test in `covers_test.go`

`TestSaveCoverFromURLGated_OfflineReturnsSentinel`:
- `setupTestDB(t)`, `withOfflineEnvDefault(t, true)`.
- Call `SaveCoverFromURLGated(dm, "https://example.invalid/cover.jpg")`.
- Assert `errors.Is(err, ErrExternalDisabled)`, no file written to
  `data/covers/` (no need to check since no HTTP attempt fires).

### Existing tests unchanged

- `TestSaveCoverFromURL_Happy`, `_HTTPError`, `_TooLarge`, `_BadMime`,
  `_PNGContentType` -- all still call the un-gated function with
  `httptest.NewServer` URLs.
- All existing handler tests for book create/update -- run with
  `offlineEnvDefault=false`, so the gate never fires; existing happy
  paths still pass.

## Implementation phasing

Single PR. Three commits in TDD order:

1. Add the gated wrapper + unit test. (`covers.go` + `covers_test.go`)
2. Update both handlers to use the gated wrapper, add `coverSkippedOffline`
   flag and conditional flash code. Add the two new flash codes.
   Add the two handler tests. (`handlers_books.go` +
   `handlers_books_test.go` + `flash.go`)
3. Optional: any additional polish surfacing during code review.

Branch name: `feat/savecoverfromurl-gate` (or similar; locked at
plan-writing time).

No DECISIONS.md update. No deployment.md update. No new bd issue
fanout (the spec already lists SSRF as out-of-scope; if Tim wants it,
file separately).

## Open follow-ups (out of scope here)

- SSRF mitigation on `SaveCoverFromURL`. Admin or crafted POST can
  submit URLs pointing at internal services (localhost, LAN, cloud
  metadata). Narrow exposure (admin-only) but worth a P3 bd issue if
  Tim wants tightening.
- Once this lands, the call-site audit for offline-mode gating is
  complete and Subproject A (Google Books) can proceed without
  needing to re-audit the existing surface.

## References

- bd issue `cs408-go-stack-di7` -- the bug report this fixes.
- DEC-033 + DEC-034 (`DECISIONS.md`) -- the offline-mode design this
  completes.
- A0 PR #78 -- shipped the predicate this hooks into.
- env-lock PR #79 -- flipped the precedence; this fix consumes the
  same `IsExternalAllowed` predicate.
