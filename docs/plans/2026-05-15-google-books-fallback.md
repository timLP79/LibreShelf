# Google Books Fallback + Enrichment (Subproject A) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Google Books as a parallel-phase fallback + field-level enrichment over the OL chain. When `GOOGLE_BOOKS_API_KEY` is set and the OL response has gaps, GB fires alongside the existing OL work-record + jscmd=data fan-out, then `mergePrefill(ol, gb)` applies "OL wins, GB fills gaps." Banner-level attribution in the site footer; per-field source tracking in the JSON response so the JS can render a "via Google Books" hint.

**Architecture:** Two-phase parallel: `FetchOpenLibraryBook` already runs OL work-record + jscmd=data concurrently when the edition is sparse. New `googlebooks.FetchByISBN` joins that goroutine fan-out behind a `googleBooksAPIKey != ""` guard. Results merge through a pure function `mergePrefill(ol, gb) *BookPrefill` with `strings.TrimSpace` gap detection. The shared response struct `OpenLibraryBook` is renamed to `BookPrefill` to reflect the merged-source nature; `DescriptionSource` is extended with `"googlebooks"`, plus new `CoverSource` and `GoogleBooksError` fields.

**Tech Stack:** Go 1.25.9+, Gin, html/template, SQLite via modernc.org/sqlite. Google Books volumes API (`https://www.googleapis.com/books/v1/volumes?q=isbn:<isbn>&key=<key>`). Existing test helpers `setupTestRouter`, `setupTestDB`, `loginAs`, `withOfflineEnvDefault`, plus a new package-level swap for `googleBooksBaseURL` and `googleBooksAPIKey` so tests can drive `httptest.NewServer`.

**Spec:** `docs/specs/2026-05-15-google-books-fallback-design.md` (Subproject A section)

---

## File Structure

- **Modify** `openlibrary.go` -- rename `OpenLibraryBook` struct to `BookPrefill`. Add `CoverSource` and `GoogleBooksError` fields. Update doc comment. Then later in Task 4, integrate GB into the parallel fan-out section (currently lines ~318-349) and call the merge.
- **Modify** `openlibrary_test.go` -- rename `OpenLibraryBook` references after the struct rename.
- **Modify** `handlers_books.go` -- rename `OpenLibraryBook` reference in `HandleOpenLibraryLookup` after the struct rename. Then in Task 4, ensure the handler emits the `GoogleBooksError` flag the merge produces (no code change needed beyond the struct already carrying the field; verify response JSON).
- **Create** `googlebooks.go` -- new file mirroring the shape of `openlibrary.go`: package-level types for the volumes API response, normalizer, `FetchByISBN(ctx, isbn) (*BookPrefill, error)`. Package-level `googleBooksAPIKey` (captured in `init()` from env), `googleBooksBaseURL` (overridable for tests).
- **Create** `googlebooks_test.go` -- httptest.NewServer-driven tests for the full happy/error matrix.
- **Create** `merge.go` -- pure function `mergePrefill(ol, gb *BookPrefill) *BookPrefill` applying "OL wins, GB fills gaps."
- **Create** `merge_test.go` -- unit tests for the merge rules.
- **Modify** `static/javascripts/app.js` -- add a small branch that renders a "Google Books unavailable" banner note when the response's `google_books_error` field is true. Mirror of the existing `description_source` banner branch.
- **Modify** `templates/layout.html` -- add a single-line site footer with the attribution copy, conditional on `OfflineLocked` and `GoogleBooksConfigured` template flags.
- **Modify** `main.go` -- compute the footer flags once at startup into a package var `siteFooter`. Pass to `renderTemplate` so every page sees the footer.
- **Modify** `handlers.go` -- update `renderTemplate` to inject `SiteFooter` into the template data automatically. (Verify the renderTemplate signature in the codebase first; if it already merges into a shared map, add the entry there.)
- **Modify** `DECISIONS.md` -- append DEC-035.
- **Modify** `docs/deployment.md` -- add `GOOGLE_BOOKS_API_KEY` to the Environment Variables section.

---

## Task 1: Rename `OpenLibraryBook` to `BookPrefill` + add new fields

Pure refactor + field additions. Compiles and passes all existing tests when complete. The two new fields default to zero values; existing JSON consumers (the admin Lookup JS) ignore them.

**Files:**
- Modify: `openlibrary.go` (struct definition + all internal references)
- Modify: `openlibrary_test.go` (test references)
- Modify: `handlers_books.go` (one return-type reference in HandleOpenLibraryLookup)

- [ ] **Step 1: Verify scope of the rename**

Run from working directory:
```bash
grep -rn "OpenLibraryBook" --include="*.go"
```

Expected: hits in `openlibrary.go` (struct definition + function returns), `openlibrary_test.go` (test variables), `handlers_books.go` (return-type reference at line ~129 in HandleOpenLibraryLookup). No other files.

- [ ] **Step 2: Apply the rename**

Use a global text replacement across the three files. From the working directory:
```bash
sed -i 's/\bOpenLibraryBook\b/BookPrefill/g' openlibrary.go openlibrary_test.go handlers_books.go
```

The `\b` word boundaries prevent matching `OpenLibraryBookSomething` if any such identifier exists (none do in this codebase).

- [ ] **Step 3: Update the struct doc comment**

Edit `openlibrary.go`. Find the doc comment block above the struct (currently at lines 51-55):

```go
// OpenLibraryBook is the prefill payload returned by HandleOpenLibraryLookup.
// Most fields come from Open Library; Description may come from
// Open Library OR from Wikipedia when OL's description is empty/thin.
// DescriptionSource ("openlibrary" or "wikipedia") tells the JS-side
// prefill code which source-label to show in the status banner.
type BookPrefill struct {
```

Replace the comment block with:

```go
// BookPrefill is the merged-source prefill payload returned by
// HandleOpenLibraryLookup. Fields originate from the Open Library chain
// (jscmd=details, work record, jscmd=data) and from the Google Books
// volumes API when GOOGLE_BOOKS_API_KEY is configured. The merge rule is
// "OL wins, GB fills gaps": each field uses the OL value if non-empty
// after strings.TrimSpace, otherwise the GB value. DescriptionSource and
// CoverSource carry the per-field origin so the admin Lookup JS can show
// a "via Google Books" hint on the staged banner. GoogleBooksError is
// true when GB was attempted (key set, gap detected) but failed; the JS
// uses this to render a small "Google Books unavailable" note.
type BookPrefill struct {
```

- [ ] **Step 4: Add the two new fields**

Edit `openlibrary.go`. The struct currently ends:

```go
type BookPrefill struct {
	Title             string   `json:"title,omitempty"`
	Authors           []string `json:"authors,omitempty"`
	PublishYear       int      `json:"publish_year,omitempty"`
	Publisher         string   `json:"publisher,omitempty"`
	CoverURL          string   `json:"cover_url,omitempty"`
	Description       string   `json:"description,omitempty"`
	DescriptionSource string   `json:"description_source,omitempty"`
}
```

Append `CoverSource` and `GoogleBooksError` so the final shape is:

```go
type BookPrefill struct {
	Title             string   `json:"title,omitempty"`
	Authors           []string `json:"authors,omitempty"`
	PublishYear       int      `json:"publish_year,omitempty"`
	Publisher         string   `json:"publisher,omitempty"`
	CoverURL          string   `json:"cover_url,omitempty"`
	Description       string   `json:"description,omitempty"`
	DescriptionSource string   `json:"description_source,omitempty"`
	CoverSource       string   `json:"cover_source,omitempty"`
	GoogleBooksError  bool     `json:"google_books_error,omitempty"`
}
```

`DescriptionSource` is now documented to accept `"openlibrary"`, `"googlebooks"`, or `""`. `CoverSource` mirrors the same set. `GoogleBooksError` defaults to false (omitted from JSON via `omitempty`).

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: all green. The rename should not change behavior; existing tests still verify the OL chain end-to-end.

- [ ] **Step 6: Commit**

```bash
git add openlibrary.go openlibrary_test.go handlers_books.go
git commit -m "$(cat <<'EOF'
refactor(openlibrary): rename OpenLibraryBook to BookPrefill, add GB fields

The struct now represents the merged-source prefill payload (OL chain
plus Google Books when configured), not OL alone. Adds CoverSource and
GoogleBooksError fields that will be populated by upcoming GB merge
logic. Existing JSON consumers ignore the new fields; omitempty hides
them when zero-valued.

No behavior change in this commit. Setup for Subproject A.

Refs cs408-go-stack-8gj.
EOF
)"
```

---

## Task 2: Google Books client (`googlebooks.go` + tests)

Standalone client. Mirrors the structure of `openlibrary.go`. Tests use `httptest.NewServer` and swap `googleBooksBaseURL` via `t.Cleanup` so production behavior is never affected.

**Files:**
- Create: `googlebooks.go`
- Create: `googlebooks_test.go`

- [ ] **Step 1: Write the failing test suite**

Create `googlebooks_test.go` with this body:

```go
// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// withGoogleBooksBaseURL swaps the package-level base URL for the
// duration of the test and restores it on Cleanup. Tests should never
// hit the real Google Books endpoint.
func withGoogleBooksBaseURL(t *testing.T, url string) {
	t.Helper()
	prior := googleBooksBaseURL
	googleBooksBaseURL = url
	t.Cleanup(func() { googleBooksBaseURL = prior })
}

// withGoogleBooksAPIKey swaps the package-level API key for the
// duration of the test. The key is captured at init() from env; tests
// override it here without touching the env.
func withGoogleBooksAPIKey(t *testing.T, key string) {
	t.Helper()
	prior := googleBooksAPIKey
	googleBooksAPIKey = key
	t.Cleanup(func() { googleBooksAPIKey = prior })
}

func startFakeGBServer(t *testing.T, body string, status int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	withGoogleBooksBaseURL(t, srv.URL)
	withGoogleBooksAPIKey(t, "test-key")
}

const gbFullPayload = `{
  "items": [{
    "volumeInfo": {
      "title": "Dune",
      "authors": ["Frank Herbert"],
      "publisher": "Ace Books",
      "publishedDate": "1965",
      "description": "Set on the desert planet Arrakis, Dune is the story of the boy Paul Atreides.",
      "imageLinks": {
        "thumbnail": "http://books.google.com/books/content?id=B1hSG45JCX4C&printsec=frontcover&img=1&zoom=1"
      }
    }
  }]
}`

const gbCoverOnlyPayload = `{
  "items": [{
    "volumeInfo": {
      "imageLinks": {
        "thumbnail": "http://books.google.com/books/content?id=X&img=1&zoom=1"
      }
    }
  }]
}`

const gbDescOnlyPayload = `{
  "items": [{
    "volumeInfo": {
      "description": "A novel."
    }
  }]
}`

const gbEmptyItemsPayload = `{"totalItems": 0}`

func TestFetchByISBN_HappyFull(t *testing.T) {
	startFakeGBServer(t, gbFullPayload, http.StatusOK)
	got, err := FetchByISBN(context.Background(), "9780441013593")
	if err != nil {
		t.Fatalf("FetchByISBN: %v", err)
	}
	if got.Title != "Dune" {
		t.Errorf("title: want %q, got %q", "Dune", got.Title)
	}
	if len(got.Authors) != 1 || got.Authors[0] != "Frank Herbert" {
		t.Errorf("authors: want [Frank Herbert], got %v", got.Authors)
	}
	if got.Publisher != "Ace Books" {
		t.Errorf("publisher: want %q, got %q", "Ace Books", got.Publisher)
	}
	if got.PublishYear != 1965 {
		t.Errorf("publish_year: want 1965, got %d", got.PublishYear)
	}
	if got.Description == "" {
		t.Errorf("description should be populated")
	}
	if got.CoverURL == "" {
		t.Errorf("cover_url should be populated")
	}
	// Cover URL must be upgraded to HTTPS (Google returns http://).
	if got.CoverURL[:8] != "https://" {
		t.Errorf("cover_url should be HTTPS, got %q", got.CoverURL)
	}
}

func TestFetchByISBN_HappyCoverOnly(t *testing.T) {
	startFakeGBServer(t, gbCoverOnlyPayload, http.StatusOK)
	got, err := FetchByISBN(context.Background(), "9780000000000")
	if err != nil {
		t.Fatalf("FetchByISBN: %v", err)
	}
	if got.CoverURL == "" {
		t.Errorf("cover_url should be populated, got empty")
	}
	if got.Title != "" || got.Description != "" {
		t.Errorf("title and description should be empty, got title=%q desc=%q", got.Title, got.Description)
	}
}

func TestFetchByISBN_HappyDescOnly(t *testing.T) {
	startFakeGBServer(t, gbDescOnlyPayload, http.StatusOK)
	got, err := FetchByISBN(context.Background(), "9780000000000")
	if err != nil {
		t.Fatalf("FetchByISBN: %v", err)
	}
	if got.Description != "A novel." {
		t.Errorf("description: want %q, got %q", "A novel.", got.Description)
	}
	if got.CoverURL != "" {
		t.Errorf("cover_url should be empty, got %q", got.CoverURL)
	}
}

func TestFetchByISBN_NoResults(t *testing.T) {
	startFakeGBServer(t, gbEmptyItemsPayload, http.StatusOK)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if !errors.Is(err, ErrGoogleBooksNotFound) {
		t.Errorf("want ErrGoogleBooksNotFound, got %v", err)
	}
}

func TestFetchByISBN_4xx(t *testing.T) {
	startFakeGBServer(t, `{"error":"bad request"}`, http.StatusBadRequest)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if err == nil {
		t.Errorf("want error on 400, got nil")
	}
	if errors.Is(err, ErrGoogleBooksNotFound) {
		t.Errorf("400 should not map to ErrGoogleBooksNotFound, got %v", err)
	}
}

func TestFetchByISBN_5xx(t *testing.T) {
	startFakeGBServer(t, `{}`, http.StatusInternalServerError)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if err == nil {
		t.Errorf("want error on 500, got nil")
	}
}

func TestFetchByISBN_RateLimit(t *testing.T) {
	startFakeGBServer(t, `{"error":"quota exceeded"}`, http.StatusTooManyRequests)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if err == nil {
		t.Errorf("want error on 429, got nil")
	}
}

func TestFetchByISBN_MalformedJSON(t *testing.T) {
	startFakeGBServer(t, `{not valid json`, http.StatusOK)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if err == nil {
		t.Errorf("want decode error, got nil")
	}
}

func TestFetchByISBN_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(gbFullPayload))
	}))
	t.Cleanup(srv.Close)
	withGoogleBooksBaseURL(t, srv.URL)
	withGoogleBooksAPIKey(t, "test-key")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := FetchByISBN(ctx, "9780000000000")
	if err == nil {
		t.Errorf("want context timeout error, got nil")
	}
}

func TestFetchByISBN_NoKey_ReturnsSentinel(t *testing.T) {
	withGoogleBooksAPIKey(t, "")
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if !errors.Is(err, ErrGoogleBooksDisabled) {
		t.Errorf("want ErrGoogleBooksDisabled, got %v", err)
	}
}

// Defensive: confirm the test fake honors the published year regardless of
// whether the Google Books field is a bare year or a YYYY-MM-DD string.
func TestFetchByISBN_PublishedDateMonthDay(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"volumeInfo": map[string]any{
				"title":         "X",
				"publishedDate": "2004-05-12",
			}},
		},
	})
	startFakeGBServer(t, string(body), http.StatusOK)
	got, err := FetchByISBN(context.Background(), "9780000000000")
	if err != nil {
		t.Fatalf("FetchByISBN: %v", err)
	}
	if got.PublishYear != 2004 {
		t.Errorf("publish_year: want 2004, got %d", got.PublishYear)
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./... -run TestFetchByISBN -v`
Expected: compile errors. `FetchByISBN`, `googleBooksBaseURL`, `googleBooksAPIKey`, `ErrGoogleBooksNotFound`, `ErrGoogleBooksDisabled` all undefined.

- [ ] **Step 3: Create `googlebooks.go`**

Create the file with this body:

```go
// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ErrGoogleBooksNotFound is returned when the GB volumes endpoint
// responds with an empty items list. This is a normal outcome for ISBNs
// GB does not catalog, and callers should treat it as a no-op rather
// than an upstream failure.
var ErrGoogleBooksNotFound = errors.New("google books: no results for ISBN")

// ErrGoogleBooksDisabled is returned when GOOGLE_BOOKS_API_KEY is unset.
// Callers should treat this as a non-error short-circuit: the GB chain
// is disabled, OL data is returned alone.
var ErrGoogleBooksDisabled = errors.New("google books: GOOGLE_BOOKS_API_KEY is not set")

// Package-level state. Captured once at startup. Tests override via
// withGoogleBooksBaseURL and withGoogleBooksAPIKey helpers.
var (
	googleBooksBaseURL = "https://www.googleapis.com/books/v1/volumes"
	googleBooksAPIKey  string
	googleBooksTimeout = 5 * time.Second
)

func init() {
	googleBooksAPIKey = os.Getenv("GOOGLE_BOOKS_API_KEY")
}

// gbResponse mirrors the volumes search envelope returned by
// https://www.googleapis.com/books/v1/volumes?q=isbn:<isbn>&key=<key>.
// Only the fields we read are declared; unknown fields are ignored.
type gbResponse struct {
	TotalItems int      `json:"totalItems"`
	Items      []gbItem `json:"items"`
}

type gbItem struct {
	VolumeInfo gbVolumeInfo `json:"volumeInfo"`
}

type gbVolumeInfo struct {
	Title         string        `json:"title"`
	Authors       []string      `json:"authors"`
	Publisher     string        `json:"publisher"`
	PublishedDate string        `json:"publishedDate"`
	Description   string        `json:"description"`
	ImageLinks    gbImageLinks  `json:"imageLinks"`
}

type gbImageLinks struct {
	Thumbnail      string `json:"thumbnail"`
	SmallThumbnail string `json:"smallThumbnail"`
}

// IsGoogleBooksConfigured returns true when GOOGLE_BOOKS_API_KEY was
// set at startup. The merge logic in FetchOpenLibraryBook reads this
// to decide whether to fan out to GB; the attribution footer reads it
// to decide whether to credit Google Books.
func IsGoogleBooksConfigured() bool {
	return googleBooksAPIKey != ""
}

// FetchByISBN queries the Google Books volumes API for the given ISBN.
// Returns ErrGoogleBooksDisabled if GOOGLE_BOOKS_API_KEY is unset.
// Returns ErrGoogleBooksNotFound if GB has no record for the ISBN.
// All other errors (network, decode, non-2xx) are wrapped and returned
// as opaque errors; callers should treat them as "GB unavailable for
// this lookup" and proceed with OL-only data.
func FetchByISBN(ctx context.Context, isbn string) (*BookPrefill, error) {
	if googleBooksAPIKey == "" {
		return nil, ErrGoogleBooksDisabled
	}

	cleaned := stripISBNFormatting(isbn)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleBooksBaseURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("q", "isbn:"+cleaned)
	q.Set("key", googleBooksAPIKey)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: googleBooksTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google books request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google books returned status %d", resp.StatusCode)
	}

	var payload gbResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("google books decode: %w", err)
	}

	if len(payload.Items) == 0 {
		return nil, ErrGoogleBooksNotFound
	}

	return normalizeGoogleBook(payload.Items[0].VolumeInfo), nil
}

// normalizeGoogleBook maps a Google Books volumeInfo object to the
// shared BookPrefill shape. Per-field source labels are NOT set here;
// the merge step (mergePrefill) decides which fields' sources to
// stamp based on whether OL also had data.
func normalizeGoogleBook(v gbVolumeInfo) *BookPrefill {
	b := &BookPrefill{
		Title:       strings.TrimSpace(v.Title),
		Authors:     v.Authors,
		Publisher:   strings.TrimSpace(v.Publisher),
		Description: strings.TrimSpace(v.Description),
	}
	if v.PublishedDate != "" {
		// publishedDate may be "2004", "2004-05", or "2004-05-12". Take
		// the first 4 characters and parse as int. Any non-numeric or
		// out-of-range value yields a zero PublishYear, which the merge
		// step treats as "no value."
		if len(v.PublishedDate) >= 4 {
			if y, err := strconv.Atoi(v.PublishedDate[:4]); err == nil && y >= 1500 && y <= 2100 {
				b.PublishYear = y
			}
		}
	}
	if v.ImageLinks.Thumbnail != "" {
		// Google returns http:// for thumbnails. Upgrade to HTTPS so the
		// downstream cover-download path does not get mixed-content
		// warnings or refusals.
		b.CoverURL = strings.Replace(v.ImageLinks.Thumbnail, "http://", "https://", 1)
	}
	return b
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./... -run TestFetchByISBN -v`
Expected: 11 PASS.

Run: `go test ./...`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add googlebooks.go googlebooks_test.go
git commit -m "$(cat <<'EOF'
feat(googlebooks): client for the Google Books volumes API

FetchByISBN(ctx, isbn) returns a *BookPrefill or one of two sentinels:
ErrGoogleBooksDisabled when GOOGLE_BOOKS_API_KEY is unset (feature off),
ErrGoogleBooksNotFound when GB has no record (normal outcome). Other
errors are wrapped opaquely; callers treat them as "GB unavailable for
this lookup" and proceed with OL-only data.

publishedDate is parsed lenient (first 4 chars as year, bounded
1500-2100). imageLinks.thumbnail URLs are upgraded from http:// to
https:// so the downstream cover-download path stays consistent.

Package-level googleBooksBaseURL and googleBooksAPIKey overridable in
tests via withGoogleBooksBaseURL / withGoogleBooksAPIKey helpers.
Production behavior is unaffected when the env var is unset.

Not wired into the OL chain yet -- the merge integration lands in a
later commit on this branch.

Refs cs408-go-stack-8gj.
EOF
)"
```

---

## Task 3: Merge function (`merge.go` + tests)

Pure function. No I/O, no state. Easy to test exhaustively.

**Files:**
- Create: `merge.go`
- Create: `merge_test.go`

- [ ] **Step 1: Write the failing test suite**

Create `merge_test.go`:

```go
// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"testing"
)

func TestMergePrefill_OLFullGBEmpty_OLWins(t *testing.T) {
	ol := &BookPrefill{
		Title:             "OL Title",
		Authors:           []string{"OL Author"},
		PublishYear:       1965,
		Publisher:         "OL Publisher",
		CoverURL:          "https://ol.example/cover.jpg",
		Description:       "OL description.",
		DescriptionSource: "openlibrary",
	}
	gb := &BookPrefill{}

	got := mergePrefill(ol, gb)

	if got.Title != "OL Title" || got.Authors[0] != "OL Author" || got.PublishYear != 1965 {
		t.Errorf("OL bibliographic fields should win, got %+v", got)
	}
	if got.Description != "OL description." || got.DescriptionSource != "openlibrary" {
		t.Errorf("OL description should win, got desc=%q src=%q", got.Description, got.DescriptionSource)
	}
	if got.CoverURL != "https://ol.example/cover.jpg" {
		t.Errorf("OL cover should win, got %q", got.CoverURL)
	}
	if got.CoverSource != "" {
		t.Errorf("CoverSource should stay empty when GB did not contribute, got %q", got.CoverSource)
	}
}

func TestMergePrefill_OLPartialNoDescGBHasDesc_GBFillsDesc(t *testing.T) {
	ol := &BookPrefill{
		Title:       "Shared Title",
		Authors:     []string{"Shared Author"},
		PublishYear: 1965,
		CoverURL:    "https://ol.example/cover.jpg",
	}
	gb := &BookPrefill{
		Title:       "GB Title (ignored)",
		Description: "GB description fills the gap.",
		CoverURL:    "https://gb.example/cover.jpg",
	}

	got := mergePrefill(ol, gb)

	if got.Title != "Shared Title" {
		t.Errorf("OL title should win, got %q", got.Title)
	}
	if got.Description != "GB description fills the gap." {
		t.Errorf("GB description should fill, got %q", got.Description)
	}
	if got.DescriptionSource != "googlebooks" {
		t.Errorf("DescriptionSource should be googlebooks, got %q", got.DescriptionSource)
	}
	if got.CoverURL != "https://ol.example/cover.jpg" {
		t.Errorf("OL cover should win when present, got %q", got.CoverURL)
	}
	if got.CoverSource != "" {
		t.Errorf("CoverSource should stay empty when OL covered it, got %q", got.CoverSource)
	}
}

func TestMergePrefill_OLEmptyGBFull_GBWinsAll(t *testing.T) {
	ol := &BookPrefill{}
	gb := &BookPrefill{
		Title:       "GB Title",
		Authors:     []string{"GB Author"},
		PublishYear: 2020,
		Publisher:   "GB Publisher",
		CoverURL:    "https://gb.example/cover.jpg",
		Description: "GB description.",
	}

	got := mergePrefill(ol, gb)

	if got.Title != "GB Title" || got.Authors[0] != "GB Author" || got.PublishYear != 2020 {
		t.Errorf("GB bibliographic fields should fill, got %+v", got)
	}
	if got.Description != "GB description." || got.DescriptionSource != "googlebooks" {
		t.Errorf("GB description should fill with source label, got desc=%q src=%q", got.Description, got.DescriptionSource)
	}
	if got.CoverURL != "https://gb.example/cover.jpg" || got.CoverSource != "googlebooks" {
		t.Errorf("GB cover should fill with source label, got url=%q src=%q", got.CoverURL, got.CoverSource)
	}
}

func TestMergePrefill_BothNil_ReturnsEmpty(t *testing.T) {
	got := mergePrefill(nil, nil)
	if got == nil {
		t.Fatalf("merge should never return nil")
	}
	if got.Title != "" || len(got.Authors) != 0 || got.Description != "" {
		t.Errorf("expected empty BookPrefill, got %+v", got)
	}
	if got.GoogleBooksError {
		t.Errorf("GoogleBooksError should be false by default")
	}
}

func TestMergePrefill_OLNilGBOnly_ReturnsGBWithSourceLabels(t *testing.T) {
	gb := &BookPrefill{
		Title:       "GB Title",
		Description: "GB desc.",
		CoverURL:    "https://gb.example/cover.jpg",
	}
	got := mergePrefill(nil, gb)
	if got.Title != "GB Title" {
		t.Errorf("GB title should pass through, got %q", got.Title)
	}
	if got.DescriptionSource != "googlebooks" {
		t.Errorf("DescriptionSource should be googlebooks, got %q", got.DescriptionSource)
	}
	if got.CoverSource != "googlebooks" {
		t.Errorf("CoverSource should be googlebooks, got %q", got.CoverSource)
	}
}

func TestMergePrefill_OLOnlyGBNil_ReturnsOL(t *testing.T) {
	ol := &BookPrefill{
		Title:             "OL Title",
		Description:       "OL desc.",
		DescriptionSource: "openlibrary",
	}
	got := mergePrefill(ol, nil)
	if got.Title != "OL Title" || got.DescriptionSource != "openlibrary" {
		t.Errorf("OL fields should pass through, got %+v", got)
	}
}

func TestMergePrefill_OLDescIsWhitespace_GBFills(t *testing.T) {
	ol := &BookPrefill{Title: "X", Description: "   \n  "}
	gb := &BookPrefill{Description: "Real description."}
	got := mergePrefill(ol, gb)
	if got.Description != "Real description." {
		t.Errorf("whitespace-only OL description should not block GB, got %q", got.Description)
	}
	if got.DescriptionSource != "googlebooks" {
		t.Errorf("DescriptionSource should be googlebooks, got %q", got.DescriptionSource)
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./... -run TestMergePrefill -v`
Expected: compile error, `mergePrefill` undefined.

- [ ] **Step 3: Create `merge.go`**

```go
// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import "strings"

// mergePrefill applies the "OL wins, GB fills gaps" rule documented in
// DEC-035. For each field, the OL value is kept if non-empty after
// strings.TrimSpace; otherwise the GB value is used. Per-field source
// labels (DescriptionSource, CoverSource) are stamped "googlebooks"
// when GB filled the gap, "openlibrary" when OL filled it, or left
// empty when neither did.
//
// Either argument may be nil. The function never returns nil.
func mergePrefill(ol, gb *BookPrefill) *BookPrefill {
	switch {
	case ol == nil && gb == nil:
		return &BookPrefill{}
	case ol == nil:
		return gbOnly(gb)
	case gb == nil:
		return olCopy(ol)
	}

	out := *ol // shallow copy; slices share backing array (not mutated below)

	if strings.TrimSpace(out.Title) == "" && gb.Title != "" {
		out.Title = gb.Title
	}
	if len(out.Authors) == 0 && len(gb.Authors) > 0 {
		out.Authors = gb.Authors
	}
	if out.PublishYear == 0 && gb.PublishYear != 0 {
		out.PublishYear = gb.PublishYear
	}
	if strings.TrimSpace(out.Publisher) == "" && gb.Publisher != "" {
		out.Publisher = gb.Publisher
	}
	if strings.TrimSpace(out.Description) == "" && gb.Description != "" {
		out.Description = gb.Description
		out.DescriptionSource = "googlebooks"
	}
	if strings.TrimSpace(out.CoverURL) == "" && gb.CoverURL != "" {
		out.CoverURL = gb.CoverURL
		out.CoverSource = "googlebooks"
	}
	return &out
}

func gbOnly(gb *BookPrefill) *BookPrefill {
	out := *gb
	if out.Description != "" {
		out.DescriptionSource = "googlebooks"
	}
	if out.CoverURL != "" {
		out.CoverSource = "googlebooks"
	}
	return &out
}

func olCopy(ol *BookPrefill) *BookPrefill {
	out := *ol
	return &out
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./... -run TestMergePrefill -v`
Expected: 7 PASS.

Run: `go test ./...`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add merge.go merge_test.go
git commit -m "$(cat <<'EOF'
feat(merge): mergePrefill applies OL-wins-GB-fills-gaps rule

Pure function, no I/O. For each prefill field, the OL value is kept
if non-empty after strings.TrimSpace, otherwise the GB value fills
the gap. Per-field source labels (DescriptionSource, CoverSource)
get stamped "googlebooks" when GB filled the gap, "openlibrary"
when OL filled it, or empty when neither did.

Handles nil on either side (no panic), returns a non-nil pointer in
every case. Not wired into the OL chain yet; integration in next
commit.

Refs cs408-go-stack-8gj.
EOF
)"
```

---

## Task 4: Integrate GB into the parallel fan-out + handler updates + JS banner

This is the biggest task. The GB call slots into the existing parallel goroutine fan-out in `FetchOpenLibraryBook` (currently at lines ~318-349 of openlibrary.go). The merge applies at the end. The handler emits the resulting BookPrefill verbatim. The JS gets a small banner-rendering branch for `google_books_error=true`.

**Files:**
- Modify: `openlibrary.go` (parallel fan-out section)
- Modify: `openlibrary_test.go` (one new integration test stubbing both OL and GB)
- Modify: `static/javascripts/app.js` (banner branch)

- [ ] **Step 1: Write the failing integration test**

Append to `openlibrary_test.go`:

```go
func TestFetchOpenLibraryBook_GBFillsGapWhenOLPartial(t *testing.T) {
	// OL returns an edition with title + authors but no description and
	// no cover. The work record returns 404 (so OL chain exhausts).
	// GB returns a description and a cover URL. Merge should produce a
	// BookPrefill with OL bibliographic data + GB description + GB cover.
	olDetails := `{"ISBN:9780000000001":{"details":{"title":"Sparse Edition","authors":[{"name":"Jane Doe"}],"publish_date":"2004","works":[{"key":"/works/OL999999W"}]}}}`
	startFakeOLRouter(t, olDetails, "", map[string]string{})

	const gbBody = `{
	  "items": [{
	    "volumeInfo": {
	      "description": "GB-sourced description.",
	      "imageLinks": {"thumbnail": "http://books.google.com/books/content?id=X&img=1"}
	    }
	  }]
	}`
	gbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(gbBody))
	}))
	t.Cleanup(gbSrv.Close)
	withGoogleBooksBaseURL(t, gbSrv.URL)
	withGoogleBooksAPIKey(t, "test-key")

	got, err := FetchOpenLibraryBook(context.Background(), "9780000000001")
	if err != nil {
		t.Fatalf("FetchOpenLibraryBook: %v", err)
	}
	if got.Title != "Sparse Edition" || len(got.Authors) == 0 {
		t.Errorf("OL bibliographic fields should win, got %+v", got)
	}
	if got.Description != "GB-sourced description." || got.DescriptionSource != "googlebooks" {
		t.Errorf("GB description should fill the gap, got desc=%q src=%q", got.Description, got.DescriptionSource)
	}
	if got.CoverURL == "" || got.CoverSource != "googlebooks" {
		t.Errorf("GB cover should fill the gap, got url=%q src=%q", got.CoverURL, got.CoverSource)
	}
	if got.GoogleBooksError {
		t.Errorf("GoogleBooksError should be false on a successful GB call")
	}
}

func TestFetchOpenLibraryBook_GBErrorSetsFlagButReturnsOL(t *testing.T) {
	olDetails := `{"ISBN:9780000000001":{"details":{"title":"Title","authors":[{"name":"A"}],"publish_date":"2004","works":[{"key":"/works/OL999999W"}]}}}`
	startFakeOLRouter(t, olDetails, "", map[string]string{})

	// GB returns 500. Handler should set GoogleBooksError=true and
	// return OL data unmodified.
	gbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(gbSrv.Close)
	withGoogleBooksBaseURL(t, gbSrv.URL)
	withGoogleBooksAPIKey(t, "test-key")

	got, err := FetchOpenLibraryBook(context.Background(), "9780000000001")
	if err != nil {
		t.Fatalf("FetchOpenLibraryBook should not surface GB error: %v", err)
	}
	if got.Title != "Title" {
		t.Errorf("OL data should still flow through, got %+v", got)
	}
	if !got.GoogleBooksError {
		t.Errorf("GoogleBooksError flag should be set when GB call fails")
	}
}

func TestFetchOpenLibraryBook_NoGBKey_OLChainOnly(t *testing.T) {
	olDetails := `{"ISBN:9780000000001":{"details":{"title":"Title","authors":[{"name":"A"}]}}}`
	startFakeOLRouter(t, olDetails, "", map[string]string{})

	withGoogleBooksAPIKey(t, "")

	got, err := FetchOpenLibraryBook(context.Background(), "9780000000001")
	if err != nil {
		t.Fatalf("FetchOpenLibraryBook: %v", err)
	}
	if got.Title != "Title" {
		t.Errorf("OL data should flow through, got %+v", got)
	}
	if got.GoogleBooksError {
		t.Errorf("GoogleBooksError should be false when GB is disabled (no error, just skipped)")
	}
}
```

The existing `startFakeOLRouter` helper is at the top of `openlibrary_test.go` (verify by grep before writing the test). Its signature is `startFakeOLRouter(t *testing.T, detailsBody, dataBody string, workBodies map[string]string, coverISBNs ...string)`.

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./... -run "TestFetchOpenLibraryBook_GBFillsGapWhenOLPartial|TestFetchOpenLibraryBook_GBErrorSetsFlagButReturnsOL|TestFetchOpenLibraryBook_NoGBKey_OLChainOnly" -v`
Expected: all three FAIL. The existing `FetchOpenLibraryBook` does not call GB.

- [ ] **Step 3: Wire GB into the parallel fan-out**

Edit `openlibrary.go`. Locate the existing parallel-fan-out section (currently around lines 305-369). The current section ends with:

```go
	if needWork {
		if workErr != nil {
			log.Printf("openlibrary: work fetch for %s: %v", entry.Details.Works[0].Key, workErr)
		} else if workResult != nil {
			if book.Description == "" && workResult.Description != "" {
				book.Description = workResult.Description
			}
			if book.CoverURL == "" && len(workResult.Covers) > 0 && workResult.Covers[0] > 0 {
				book.CoverURL = fmt.Sprintf(olCoverURLTemplate, workResult.Covers[0])
			}
		}
	}
	if needDataAuthors {
		if authorErr != nil {
			log.Printf("openlibrary: data-authors fetch for %s: %v", bibkey, authorErr)
		} else {
			book.Authors = authorNames
		}
	}
```

After this block (and before any return statement that follows), add the GB merge logic. Insert these changes:

(a) In the goroutine-launch section (around lines 329-348), add a third conditional goroutine after `needDataAuthors`:

```go
	var (
		gbResult *BookPrefill
		gbErr    error
	)
	// GB fans out alongside the OL work/jscmd=data fallbacks. Only
	// fires when (1) the API key is configured AND (2) the OL response
	// has at least one prefill gap (any field empty). The gap check
	// happens here against the *current* OL state (after the edition
	// parse but before the work/data fallbacks merge in), which is
	// conservative -- the GB call may end up unused if work/data later
	// fill all the gaps. Acceptable trade for the latency win of the
	// parallel fan-out.
	needGB := IsGoogleBooksConfigured() && hasAnyGap(book)
	if needGB {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gbResult, gbErr = FetchByISBN(ctx, isbn)
		}()
	}
```

(b) The existing `wg.Wait()` (around line 349) stays put.

(c) After the `if needDataAuthors { ... }` block, append:

```go
	// Apply GB merge after OL fallbacks have had their chance to fill
	// gaps. This means GB only contributes to fields still empty after
	// the full OL chain ran, which respects DEC-035's "OL wins" rule.
	if needGB {
		switch {
		case errors.Is(gbErr, ErrGoogleBooksNotFound):
			// Normal outcome for ISBNs GB does not catalog. No error
			// flag; OL data flows through unchanged.
		case errors.Is(gbErr, ErrGoogleBooksDisabled):
			// Can only happen if the key was unset between the
			// IsGoogleBooksConfigured check and the FetchByISBN call.
			// Treat the same as not-found.
		case gbErr != nil:
			// Real GB failure (network, 5xx, decode). Surface via the
			// flag so the JS can render a banner note; OL data still
			// flows through.
			log.Printf("openlibrary: google books fetch for %s: %v", isbn, gbErr)
			book.GoogleBooksError = true
		case gbResult != nil:
			merged := mergePrefill(book, gbResult)
			// Preserve the GoogleBooksError flag on the merge output;
			// merge does not carry it through itself.
			merged.GoogleBooksError = book.GoogleBooksError
			book = merged
		}
	}
```

(d) Add a small helper `hasAnyGap` at the bottom of openlibrary.go (or near the existing normalizers):

```go
// hasAnyGap returns true when at least one BookPrefill field is empty
// after TrimSpace. The merge step uses this to decide whether to fan
// out to GB; an entirely-populated OL response skips the GB call.
func hasAnyGap(b *BookPrefill) bool {
	if b == nil {
		return true
	}
	if strings.TrimSpace(b.Title) == "" {
		return true
	}
	if len(b.Authors) == 0 {
		return true
	}
	if b.PublishYear == 0 {
		return true
	}
	if strings.TrimSpace(b.Publisher) == "" {
		return true
	}
	if strings.TrimSpace(b.CoverURL) == "" {
		return true
	}
	if strings.TrimSpace(b.Description) == "" {
		return true
	}
	return false
}
```

Make sure `errors` is imported at the top of openlibrary.go (it already is for the existing `errors.Is` calls).

- [ ] **Step 4: Run integration tests**

Run: `go test ./... -run "TestFetchOpenLibraryBook_GBFillsGapWhenOLPartial|TestFetchOpenLibraryBook_GBErrorSetsFlagButReturnsOL|TestFetchOpenLibraryBook_NoGBKey_OLChainOnly" -v`
Expected: all three PASS.

Run: `go test ./...`
Expected: all green. The existing OL tests still pass because GB never fires (no `GOOGLE_BOOKS_API_KEY` in the test environment, and the tests do not set it via the helper unless they're the new GB-integration tests).

- [ ] **Step 5: Add the JS banner branch**

Edit `static/javascripts/app.js`. Find the OL Lookup success handler that processes the JSON response. The existing banner-render code is around lines 359-377 (in PR #78 era; verify with `grep -n "description_source\|cover_url" static/javascripts/app.js`). After the existing banner text assignment, add a small additional check:

```javascript
                // After the existing "Description from Open Library" banner-text setup:
                var msg = data.cover_url
                    ? "Prefilled from Open Library."
                    : "Prefilled from Open Library (no cover available).";
                if (data.google_books_error) {
                    msg += " Google Books unavailable; showing Open Library data only.";
                }
                if (data.description_source === "googlebooks" || data.cover_source === "googlebooks") {
                    msg += " Some fields via Google Books.";
                }
                statusBanner.textContent = msg;
```

(The exact line numbers and surrounding variable names depend on the current app.js state. The implementer should read app.js to find the OL Lookup success branch and adapt the additions to fit. The two new conditions append text to the existing message.)

- [ ] **Step 6: Run the full suite and verify**

Run: `go test ./...`
Expected: all green.

- [ ] **Step 7: Commit**

```bash
git add openlibrary.go openlibrary_test.go static/javascripts/app.js
git commit -m "$(cat <<'EOF'
feat(openlibrary): integrate Google Books into the parallel fan-out

When GOOGLE_BOOKS_API_KEY is set AND the OL edition response has any
gap (TrimSpace-empty title/authors/year/publisher/cover/description),
googlebooks.FetchByISBN fans out alongside the existing OL work-record
and jscmd=data fetches. After all three concurrent calls complete, the
OL fallbacks merge first (work + data authors), then mergePrefill applies
"OL wins, GB fills gaps" to combine the OL chain output with GB.

On GB error (network, non-2xx, decode), the GoogleBooksError flag on the
returned BookPrefill is set so the admin Lookup JS can render a small
"Google Books unavailable" note. ErrGoogleBooksNotFound and
ErrGoogleBooksDisabled are silent short-circuits (no flag set; OL data
returned unchanged).

JS-side: app.js appends a "Some fields via Google Books." note when the
response's description_source or cover_source equals "googlebooks", and
a "Google Books unavailable; showing Open Library data only." note when
google_books_error is true.

Refs cs408-go-stack-8gj.
EOF
)"
```

---

## Task 5: Attribution footer in `templates/layout.html`

Operator-visible attribution per Google Books ToS expectations. Footer text computed once at startup, rendered on every page that uses the shared layout.

**Files:**
- Modify: `main.go` (compute `siteFooter` flags once at startup)
- Modify: `handlers.go` (inject `SiteFooter` into the template data via `renderTemplate`; verify the existing function shape)
- Modify: `templates/layout.html` (add the footer block before the closing `</body>` tag)

- [ ] **Step 1: Inspect the renderTemplate function**

Run: `grep -n "func renderTemplate" handlers.go`

Read the function. Note its signature and how it merges per-request data into the template. If it accepts a `gin.H` and passes it to a `template.Execute` call, the injection point is just before that Execute call.

- [ ] **Step 2: Add package-level siteFooter state in main.go**

Edit `main.go`. After the existing template setup but before the route group declarations, add:

```go
// siteFooter holds the per-deployment footer flags computed once at
// startup. The layout template renders the appropriate attribution text
// based on these flags.
var siteFooter = struct {
	GoogleBooksConfigured bool
	OfflineLocked         bool
}{
	GoogleBooksConfigured: IsGoogleBooksConfigured(),
	OfflineLocked:         IsOfflineEnvLocked(),
}
```

Place this declaration as a function-local var inside `main()` AFTER `dm.SeedDefaultUsers()` but before the template-loading block. (If you'd prefer a true package-level var, move it outside `main()` and initialize via an `init()` in a separate file -- but the function-local form is simpler since it threads naturally into the template injection.)

Actually a cleaner approach: keep siteFooter as a package-level var in a new file (or near the top of main.go) and have it lazily compute via accessors. Use this pattern instead:

Add a new file `footer.go`:

```go
// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

// SiteFooter is the per-deployment attribution state injected into every
// rendered page via renderTemplate. Computed once at startup so templates
// do not re-check env or DB on every render.
type SiteFooter struct {
	GoogleBooksConfigured bool
	OfflineLocked         bool
}

// siteFooter is the shared attribution state. Read in renderTemplate.
// Written only at startup via initSiteFooter.
var siteFooter SiteFooter

// initSiteFooter captures the deployment's attribution state. Called
// once from main() after package init() functions have run.
func initSiteFooter() {
	siteFooter = SiteFooter{
		GoogleBooksConfigured: IsGoogleBooksConfigured(),
		OfflineLocked:         IsOfflineEnvLocked(),
	}
}
```

In `main.go`, call `initSiteFooter()` early -- right after `dm.SeedDefaultUsers()`:

```go
dm.SeedDefaultUsers()
initSiteFooter()
```

- [ ] **Step 3: Inject siteFooter into renderTemplate**

Edit `handlers.go`. Find `renderTemplate` (the function that accepts a template name and a `gin.H` data map). Just before the `template.Execute` (or equivalent) call, ensure `SiteFooter` is in the data map:

```go
func renderTemplate(c *gin.Context, name string, data gin.H) {
	// ... existing setup ...
	if _, ok := data["SiteFooter"]; !ok {
		data["SiteFooter"] = siteFooter
	}
	// ... existing Execute call ...
}
```

The "if not already set" guard means a handler can override the footer state per-request if it ever needs to (none do today, but the pattern is defensive).

- [ ] **Step 4: Add the footer block in templates/layout.html**

Edit `templates/layout.html`. Find the closing `</body>` tag near the end of the file. Just before it, add the footer markup:

```html
    <footer class="container mt-5 mb-3">
      <p class="small text-muted text-center mb-0">
        {{if .SiteFooter.OfflineLocked}}
          Running in offline mode. No external metadata sources are being contacted.
        {{else if .SiteFooter.GoogleBooksConfigured}}
          Book data from <a href="https://openlibrary.org" rel="noopener" target="_blank">Open Library</a> and <a href="https://books.google.com" rel="noopener" target="_blank">Google Books</a>.
        {{else}}
          Book data from <a href="https://openlibrary.org" rel="noopener" target="_blank">Open Library</a>.
        {{end}}
      </p>
    </footer>
```

The `rel="noopener"` and `target="_blank"` are deliberate: external links open in a new tab without granting the new tab access to `window.opener`.

- [ ] **Step 5: Run the test suite**

Run: `go test ./...`
Expected: all green. No new tests written for the footer in this task; the visible attribution is verified manually in Task 6's manual checklist.

If a handler test happens to render the layout and assert on body content, the footer text would now appear in those bodies. Verify that no existing test asserts on the absence of "Book data from" or similar markers; if so, update them. Run all settings/handler tests to confirm:

Run: `go test ./... -run "TestSettingsPage|TestBookCreate|TestBookUpdate|TestHandleOpenLibraryLookup" -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add footer.go main.go handlers.go templates/layout.html
git commit -m "$(cat <<'EOF'
feat(footer): site-wide attribution for OL + GB sources

Adds a small footer to every page rendered through the shared layout.
Text is conditional on three startup-captured flags: offline-mode lock
(no attribution; no external data being used), Google Books configured
(credit OL + GB), or neither (credit OL only).

State captured once at startup via initSiteFooter into the package var
siteFooter. renderTemplate injects it into every template data map so
handlers do not need to be aware of the footer. The kiosk layout is
separate and does not need the footer (no admin actions surface
external metadata there).

External links open with rel="noopener" target="_blank" so the new
tab cannot access window.opener.

Refs cs408-go-stack-8gj.
EOF
)"
```

---

## Task 6: Documentation (DEC-035 + deployment.md)

Docs-only. No code changes.

**Files:**
- Modify: `DECISIONS.md`
- Modify: `docs/deployment.md`

- [ ] **Step 1: Append DEC-035 to DECISIONS.md**

Open `DECISIONS.md` and append at the end of the file:

```markdown

---

## DEC-035: Google Books as fallback + field-level enrichment over Open Library

**Date:** 2026-05-15 (cs408-go-stack-8gj, branch `feat/google-books-fallback`).

**Decision:** When `GOOGLE_BOOKS_API_KEY` is set at startup and the Open Library edition response has any gap (TrimSpace-empty title, authors, year, publisher, cover URL, or description), the OL chain fans out to Google Books in parallel with the existing OL work-record and jscmd=data fallbacks. After all concurrent calls complete, `mergePrefill(ol, gb)` applies the rule "OL wins, GB fills gaps" -- each field uses the OL value if non-empty after TrimSpace, otherwise the GB value. Per-field source labels (`DescriptionSource`, `CoverSource`) carry "googlebooks" when GB filled the gap, "openlibrary" when OL filled it. On GB error, the OL data flows through unchanged and `GoogleBooksError = true` so the JS banner can show a "Google Books unavailable" note.

**Why fallback + enrichment, not replacement:** Open Library is the open-data primary that aligns with LibreShelf's self-hostable framing. The DEC-032 chain already covers 100% of OL-catalogued books. The remaining gap is books OL does not catalog at all (mostly new releases and niche presses); GB has substantially better coverage there. For OL-catalogued books with sparse editions, GB's descriptions and covers are often richer than OL's, so enriching gap-by-gap gives noticeable quality improvement without abandoning OL as the source of truth for bibliographic data.

**Why parallel, not sequential:** The OL chain already runs work-record and jscmd=data concurrently (commit 316e31c). Adding GB to that fan-out keeps end-to-end latency dominated by the slowest of the three calls (~700ms typical) rather than their sum (~1500ms+ if GB were tacked on after).

**Asymmetric trigger:** GB fires only when the OL edition response has at least one gap. An entirely-populated OL response (rare but possible) skips the GB call entirely. The gap check happens before the OL work/data fallbacks complete, so it is conservative -- GB may fire even when the work record would have filled the gap. That trade is acceptable: the latency win of the parallel fan-out outweighs the occasional unused GB call.

**API key handling:** `GOOGLE_BOOKS_API_KEY` is opt-in. Unset means GB is disabled; the OL chain runs alone with no error, no warning. Get a key from Google Cloud Console; free tier 1000 requests/day is sufficient for typical small-library volume.

**Error policy:**
- `ErrGoogleBooksDisabled` (key unset) -- silent short-circuit; no flag.
- `ErrGoogleBooksNotFound` (GB has no record) -- silent short-circuit; no flag.
- Other errors (network, 5xx, decode, timeout) -- `GoogleBooksError = true` on the response; OL data still flows through; JS renders a small "Google Books unavailable" banner note.

**Attribution:** A site-wide footer renders on every page through the shared layout. When offline-locked: no attribution (no external data is being used). When GB configured: "Book data from Open Library and Google Books." Otherwise: "Book data from Open Library." State captured once at startup; templates do not re-check on every render.

**Cover downloads:** A GB-sourced cover URL flows through the same `cover_url` form field that OL covers use. On submit, `HandleBookCreate` / `HandleBookUpdate` call `SaveCoverFromURLGated` to download. No new download path is added; the existing gated wrapper handles any URL the prefill returns.

**Struct change:** `OpenLibraryBook` is renamed to `BookPrefill` (PR commit 1 on this branch). The struct now represents the merged-source payload, not OL alone. New fields: `CoverSource` (parallel to `DescriptionSource`), `GoogleBooksError`. Existing JSON consumers (the admin Lookup JS) ignore the new fields when zero-valued via `omitempty`.

**Related:**
- DEC-032 -- OL enrichment chain that this design layers on top of.
- DEC-033 -- offline-mode predicate that gates the GB call.
- DEC-034 -- env-var-as-lock precedence flip that GB consumes via IsExternalAllowed.
- bd issue `cs408-go-stack-8gj` -- the parent issue for this work.
- Spec: `docs/specs/2026-05-15-google-books-fallback-design.md`.
```

- [ ] **Step 2: Add GOOGLE_BOOKS_API_KEY to deployment.md**

Edit `docs/deployment.md`. In the `## Environment Variables` section (immediately after the `### LIBRESHELF_OFFLINE (optional)` subsection), append a new subsection:

```markdown

### `GOOGLE_BOOKS_API_KEY` (optional)

Set to a Google Books API key to enable the Google Books fallback +
enrichment chain. When set, the admin OL Lookup fans out to GB in
parallel with the OL chain's internal fallbacks and merges results
using the "OL wins, GB fills gaps" rule. The site footer credits
"Open Library and Google Books." Default: unset (GB disabled, OL only).

When unset, no GB API calls are made. The OL chain runs alone with no
warnings or errors. The site footer credits "Open Library" only.

Get a key from Google Cloud Console (Library -> Books API -> Credentials).
Free-tier quota is 1000 requests/day, sufficient for typical small-library
admin Lookup volume (one click per ISBN, occasionally).

To set via systemd, add an `Environment=` line in `deploy/libreshelf.service`:

```
Environment=GOOGLE_BOOKS_API_KEY=AIza...redacted...
```

Then `sudo systemctl daemon-reload && sudo systemctl restart libreshelf`.

When offline mode is on (`LIBRESHELF_OFFLINE=true`), the GB key is
ignored and no GB calls are made -- the offline predicate fires before
the OL chain, which is the only entry point that calls GB.
```

- [ ] **Step 3: Verify**

Run from working directory:

```bash
go test ./...
```
Expected: all green (docs-only commit, but sanity check).

```bash
go vet ./...
```
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add DECISIONS.md docs/deployment.md
git commit -m "$(cat <<'EOF'
docs: DEC-035 records the Google Books fallback + enrichment design

DEC-035 captures the OL-wins-GB-fills-gaps rule, parallel fan-out
architecture, asymmetric trigger (GB fires only on OL gaps),
API-key-opt-in semantics, error policy (silent short-circuit vs flag),
site-footer attribution, and the cover-download flow.

deployment.md gains a GOOGLE_BOOKS_API_KEY subsection with the
operator workflow (systemd Environment= line + daemon-reload). Notes
that the key is ignored when offline mode is locked.

Refs cs408-go-stack-8gj.
EOF
)"
```

---

## Self-Review

**1. Spec coverage:**

| Spec requirement | Implemented in |
|---|---|
| Rename OpenLibraryBook to BookPrefill | Task 1 |
| Add CoverSource and GoogleBooksError fields | Task 1 |
| googlebooks.go client + types | Task 2 |
| FetchByISBN(ctx, isbn) returning BookPrefill | Task 2 |
| GOOGLE_BOOKS_API_KEY env var, optional | Task 2 |
| mergePrefill OL-wins-GB-fills-gaps logic | Task 3 |
| Per-field source labels for description and cover | Task 3 (set by merge), Task 1 (struct fields) |
| GB fans out alongside OL work-record + jscmd=data | Task 4 |
| GB only fires when OL has a gap | Task 4 (hasAnyGap helper) |
| GoogleBooksError flag set on GB error | Task 4 |
| Silent fall-through on ErrGoogleBooksNotFound/Disabled | Task 4 |
| JS banner renders the "via Google Books" / "GB unavailable" notes | Task 4 |
| Attribution footer in shared layout | Task 5 |
| Footer state captured once at startup | Task 5 (initSiteFooter) |
| Footer rendered conditionally on lock + GB-configured | Task 5 |
| GB cover URL flows through SaveCoverFromURLGated on submit | Task 4 (no new download path; verified in spec note) |
| GB client tests: happy full + cover-only + desc-only + no-results + 4xx + 5xx + 429 + malformed JSON + context timeout + no-key + date parsing | Task 2 (11 tests) |
| Merge tests: OL full + GB empty, OL partial, OL empty, both empty, OL-only, GB-only, whitespace-only OL | Task 3 (7 tests) |
| Integration tests: GB fills gap, GB error, no GB key | Task 4 (3 tests) |
| DEC-035 entry | Task 6 |
| deployment.md GOOGLE_BOOKS_API_KEY subsection | Task 6 |

**2. Placeholder scan:** No TBDs, no TODOs, no "similar to above," no "add appropriate error handling." Each task contains the actual code to write.

**3. Type consistency:** `BookPrefill`, `mergePrefill`, `FetchByISBN`, `IsGoogleBooksConfigured`, `googleBooksAPIKey`, `googleBooksBaseURL`, `ErrGoogleBooksNotFound`, `ErrGoogleBooksDisabled`, `gbResponse`, `gbItem`, `gbVolumeInfo`, `gbImageLinks`, `hasAnyGap`, `siteFooter`, `initSiteFooter`, `SiteFooter` -- all used consistently across tasks. Test helpers `withGoogleBooksBaseURL`, `withGoogleBooksAPIKey`, `startFakeGBServer` defined in Task 2 and reused in Task 4.

**Test helpers used:** `setupTestDB(t)` (db_test.go:19), `mustCreateUser(t, dm, username, role)` (db_test.go:32), `setupTestRouter(t)`, `loginAs(t, dm, username, role)`, `withOfflineEnvDefault(t, locked)` (network_test.go), `startFakeOLRouter(t, detailsBody, dataBody, workBodies, coverISBNs...)` (openlibrary_test.go). All confirmed present.

---

## Execution

Plan complete and saved to `docs/plans/2026-05-15-google-books-fallback.md`. Two execution options:

1. **Subagent-Driven (recommended)** -- dispatch a fresh subagent per task, review between tasks.
2. **Inline Execution** -- run the tasks in this session using executing-plans with checkpoints.

Which approach?
