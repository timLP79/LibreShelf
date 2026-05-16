# Google Books Integration + Offline Mode (Subprojects A0 + A)

Status: Design approved 2026-05-15. Implementation plan to follow.
Owner: Tim Palacios
Related bd issues: cs408-go-stack-8gj (Google Books fallback), cs408-go-stack-0eh (offline workflow context)
Supersedes (partially): cs408-go-stack-069 (OL OLID/LCCN/OCLC exhaustion). Re-evaluate after A lands.

## Context

DEC-032 (2026-05-13) reorganized the Open Library enrichment chain so it
chases editions through OL's own work records, jscmd=data, and the ISBN
cover endpoint. The current chain recovers covers and descriptions for
100/100 seed books that OL catalogs. The remaining metadata gap is books
OL does not catalog at all (new releases, niche presses, some
non-English titles).

This spec adds Google Books as a secondary metadata source. It runs as
both a total-miss fallback and a field-level enrichment over partial OL
hits. Because the same integration introduces an external dependency on
a second commercial API, the spec also covers an operator-declared
offline mode that gates all external calls (OL and GB) for deployments
where internet access is unavailable or policy-restricted (e.g. prisons,
secure facilities).

The work is sliced into four sequential subprojects. This document
covers the first two:

- **A0** -- Offline mode declaration and predicate.
- **A** -- Google Books client and merge, wired into the existing
  single-add OL Lookup.
- B (later, separate spec) -- Side-by-side picker UI for single-add.
- C (later, separate spec) -- Bulk book import.

## Goals

- Operator can declare a LibreShelf deployment is offline. While offline,
  no external HTTP calls are made by OL Lookup, the seed-cover backfill,
  or any future external source.
- When online and Google Books is configured, OL Lookup transparently
  enriches partial OL records with Google Books data for empty fields.
- When online and OL returns no record at all, Google Books can supply a
  full prefill.
- Google Books integration is opt-in via env var. Unset = feature absent,
  no surprise external dependency.
- Both APIs are presented in a global "Powered by" footer when in use,
  consistent with Google Books ToS attribution expectations.

## Non-goals

- Auto-detecting offline state. Detection from inside the app is
  unreliable for restricted networks; operator declares it.
- Per-record source columns on the books table. Global footer covers
  attribution; per-record tracking is unnecessary complexity.
- Side-by-side OL/GB picker for single-add. Deferred to subproject B.
- Bulk book import. Deferred to subproject C, which will reuse this
  spec's API client.
- Rate-limit telemetry for Google Books. Single-add traffic is too low
  to bother; reconsider in subproject C.
- Google Books as a primary source. OL stays primary to preserve
  open-data alignment.

## Subproject A0: Offline Mode Declaration

### Decision

Offline mode is a boolean predicate with two declaration sources:

1. Startup env var `LIBRESHELF_OFFLINE` (default false).
2. Admin-only runtime override stored in the existing `settings` table
   under key `offline_mode`.

Precedence: if the settings table row exists, it wins over the env var.
This lets a connected library flip offline temporarily during a network
outage without restart, and lets an offline-by-default deployment stay
offline even if the env var were misconfigured.

### Surfaces

- **Env var:** `LIBRESHELF_OFFLINE` parsed once at startup in `main.go`.
  Documented in `deploy/env.example` (create if it does not exist) and
  in `docs/deployment.md`.
- **Settings row:** read via existing `GetSettingBool("offline_mode",
  envDefault)`. Written via existing `SetSetting`.
- **Admin Settings UI:** `templates/admin_settings.html` gains a checkbox
  "Disable external API calls (offline mode)" under a new section.
  Handler logic mirrors the existing `staff_can_import_patrons` toggle
  in `handlers_settings.go`. Only admins can reach this page (existing
  route guard).
- **Patron / staff UI:** no change. Offline mode is an operator concern.

### Predicate API

New function in a new file `network.go`:

```go
// IsExternalAllowed returns true when external HTTP calls (Open Library,
// Google Books, future sources) are permitted for this deployment.
// Reads the offline_mode settings row if present, falling back to the
// LIBRESHELF_OFFLINE env var captured at startup.
func IsExternalAllowed(dm *DatabaseManager) bool { ... }
```

Startup default is captured into a package var so the predicate does not
re-read os.Getenv on every call. The settings table read is cheap (key-value lookup)
and tolerates the table being unreachable: on DB error the function logs
and returns the env-var default, fail-closed if env says offline,
fail-open if env says online. The OL chain and seed backfill already
tolerate transient lookup failures, so this matches their posture.

### Call sites wired in A0

- `FetchOpenLibraryBook` (and any other exported OL entry points in
  `openlibrary.go`): check predicate first, return a sentinel error
  `ErrExternalDisabled` if false.
- `FetchAndStoreSeedCovers` (seed backfill, `main.go` or `covers.go`):
  check predicate before the per-book loop. If false, log once
  ("offline mode: skipping seed cover backfill") and return without
  attempting any HTTP.
- `HandleOpenLibraryLookup` (`handlers_books.go`): if predicate is
  false, respond with 503 and a JSON body the admin Lookup JS can show
  as a banner ("Offline mode: external lookups disabled").

### Tests for A0

- Unit: predicate returns env default when settings row absent.
- Unit: predicate returns settings value when row present (covers both
  directions of override).
- Unit: predicate returns env default on simulated DB error and logs.
- Handler: `HandleOpenLibraryLookup` with offline mode on returns 503
  with the expected banner JSON.
- Backfill: with offline mode on, `FetchAndStoreSeedCovers` exits
  without any HTTP attempts (no `httptest.NewServer` activity, asserted
  via call count on a stubbed transport or by inspecting the cover
  storage state).
- Settings flow: POST to settings handler with `offline_mode=on` flips
  the row; subsequent predicate read reflects the change.

## Subproject A: Google Books Client and Merge

Depends on A0. The merge runs only when the predicate allows external
calls.

### Decision summary

- Goal: total-miss fallback plus field-level enrichment over partial OL
  hits.
- API key handling: opt-in via `GOOGLE_BOOKS_API_KEY` env var. Unset =
  feature off, OL chain runs alone, no errors.
- Trigger: field-level. Google Books fires whenever any OL prefill
  field is empty after the OL chain completes its own internal
  fallbacks.
- Merge rule: "OL wins, GB fills gaps." For each field in the prefill
  set, OL value wins if non-empty after `strings.TrimSpace`; otherwise
  Google Books value is used.
- Error handling: silent fall-through with a small banner note in the
  Lookup modal ("Google Books unavailable, showing Open Library data
  only").
- Attribution: global footer text in the base layout. Conditional on
  GB key being set and offline mode being off.

### Architecture: two-phase parallel

The existing OL chain already runs a parallel phase when the first
edition response is sparse (commit 316e31c). Google Books joins that
same phase rather than running serially after it.

```
HandleOpenLibraryLookup(isbn)
  |
  +-- IsExternalAllowed(dm)? -- no --> 503 (banner)
  |   yes
  |
  +-- FetchOpenLibraryBook(ctx, isbn)
  |     |
  |     +-- OL ISBN call (jscmd=details)
  |     |
  |     +-- if sparse, run in parallel:
  |           +-- OL work record fetch
  |           +-- OL jscmd=data
  |           +-- googlebooks.FetchByISBN(ctx, isbn)
  |                   (only if GOOGLE_BOOKS_API_KEY is set)
  |
  +-- Merge in Go: OL wins, GB fills gaps
  |
  +-- Return BookPrefill + GoogleBooksError flag
```

Latency in the common partial-OL case is dominated by the slowest of
the three parallel calls, not their sum. Google Books is never called
when the OL ISBN response is already complete (no parallel phase
triggered).

### Files

- New `googlebooks.go` mirroring the shape of `openlibrary.go`:
  - Package-level types for the Google Books API response (volumes
    search by ISBN).
  - Normalizer that maps a raw response to the same shape used by the
    OL normalizer.
  - `FetchByISBN(ctx context.Context, isbn string) (*googleBook, error)`.
  - HTTP client uses the context timeout passed in; no global timeout.
- New `googlebooks_test.go` with `httptest.NewServer` stubs, mirroring
  `openlibrary_test.go`.
- Rename `OpenLibraryBook` struct in `openlibrary.go` to `BookPrefill`.
  Update all references. The struct now represents the merged OL+GB
  prefill payload, not OL alone. Comment block updated to reflect.
- New `merge.go` (or merge functions inside `openlibrary.go` if small):
  `mergePrefill(ol *BookPrefill, gb *googleBook) *BookPrefill` applying
  "OL wins, GB fills gaps" with TrimSpace gap detection.
- `handlers_books.go`: `HandleOpenLibraryLookup` updated to call the
  merge and to emit the `GoogleBooksError` flag on partial failure.

### Struct changes

`BookPrefill` (renamed from `OpenLibraryBook`):

- Existing fields preserved: Title, Authors, PublishYear, Publisher,
  CoverURL, Description, DescriptionSource.
- `DescriptionSource` now accepts a third value: `"googlebooks"`. Other
  values: `"openlibrary"` and empty.
- New field `CoverSource` mirroring DescriptionSource semantics so the
  Lookup modal can show "Cover via Google Books" when relevant. Stored
  in JSON only; not persisted to the books table.
- New field `GoogleBooksError bool` so the JS knows to render the
  unavailable banner.

### Attribution footer

Implemented as a new footer block in `templates/layout.html` (the
shared base layout; verified it has no existing footer to conflict
with). Small footer line:

- Online + GB key set: "Book data from Open Library and Google Books."
- Online + no GB key: "Book data from Open Library."
- Offline: no attribution text (no external data is being used).

Footer config is computed once at startup into a package struct
(`siteFooter` or similar) so templates do not re-check env on every
render.

### Tests for A

- Google Books client unit tests (httptest.NewServer):
  - Happy path with full data.
  - Happy path with cover only.
  - Happy path with description only.
  - No results found (empty `items`).
  - 4xx response.
  - 5xx response.
  - 429 rate limit response.
  - Malformed JSON.
  - Context timeout.
- Merge unit tests:
  - OL full + GB empty: result equals OL.
  - OL partial (no description) + GB has description: GB description
    used; DescriptionSource = `"googlebooks"`.
  - OL empty + GB full: result equals GB; sources all `"googlebooks"`.
  - Both empty: empty BookPrefill, no error.
  - OL hit + GB error: result equals OL, `GoogleBooksError = true`.
- Predicate integration: with offline mode on, no GB HTTP call attempted
  even if `GOOGLE_BOOKS_API_KEY` is set.
- No-key integration: with `GOOGLE_BOOKS_API_KEY` unset, OL Lookup
  returns OL-only data with no error flag.
- Handler integration: `HandleOpenLibraryLookup` against stubbed OL +
  stubbed GB returns the expected merged JSON shape.

### Operational notes for `deploy/env.example` and `docs/deployment.md`

- `GOOGLE_BOOKS_API_KEY` -- optional. Enables the Google Books
  fallback/enrichment when set. Get a key from Google Cloud Console.
  Free tier 1000/day.
- `LIBRESHELF_OFFLINE` -- optional, default false. Set to `true` to
  disable all external API calls. Admin can also flip at runtime via
  Settings.

## Implementation phasing

Two PRs, sequential.

1. **PR 1 -- A0.** Predicate, settings row, admin toggle, wired into
   existing OL chain entry points and seed-cover backfill. Tests
   listed in A0 section. No new external API surface; existing
   behavior unchanged when offline mode stays off.

2. **PR 2 -- A.** Google Books client, struct rename to BookPrefill,
   merge logic, parallel-phase integration, attribution footer, banner
   note for GB errors. Tests listed in A section.

Both PRs add a new DECISIONS.md entry:
- A0 -> DEC-033 (Offline mode declaration).
- A -> DEC-034 (Google Books fallback and enrichment).

## Open questions / future work

- Subproject B (picker UI) and C (bulk import) get their own brainstorms
  and specs.
- Once A lands, re-evaluate cs408-go-stack-069 (OL OLID/LCCN/OCLC
  exhaustion). Google Books often covers what those paths would have,
  so #069 may close as superseded.
- Future external sources (Internet Archive, Wikidata, placeholder
  generators) would reuse the same offline predicate. No additional
  cross-cutting work needed.

## References

- DEC-032 -- Open Library enrichment chain.
- cs408-go-stack-8gj -- bd issue: Google Books fallback.
- cs408-go-stack-0eh -- bd issue: offline workflow context.
- cs408-go-stack-069 -- bd issue: OL endpoint exhaustion (re-evaluate).
- `openlibrary.go`, `handlers_books.go`, `handlers_settings.go` -- the
  files this design extends.
