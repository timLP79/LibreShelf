# LibreShelf -- Design Decisions

Architectural and design decisions made during development. Each entry records the context,
the decision, and the reasoning so future sessions start with full understanding.

---

## DEC-001: Bootstrap served locally, not via CDN

**Date:** 2026-02-01 (CP1)
**Context:** The product specification listed "Bootstrap 5 via CDN." The app is designed to be
self-hostable and offline-capable.
**Decision:** Serve Bootstrap from `static/stylesheets/` and `static/javascripts/` instead of a CDN.
**Rationale:** Eliminates CDN supply chain risk and supports offline-first architecture. The spec
PDF cannot be changed but this divergence is intentional.

---

## DEC-002: Pure-Go SQLite driver (no CGo)

**Date:** 2026-02-01 (CP1)
**Context:** Two main SQLite options in Go: `mattn/go-sqlite3` (CGo) and `modernc.org/sqlite` (pure Go).
**Decision:** Use `modernc.org/sqlite`, which registers as driver `"sqlite"` (not `"sqlite3"`).
**Rationale:** No CGo means no C compiler dependency, simpler cross-compilation, and easier
deployment. Pure Go binary with zero system library requirements.

---

## DEC-003: Flat package structure

**Date:** 2026-02-01 (CP1)
**Context:** Go projects can use sub-packages (`internal/handlers/`, `internal/db/`, etc.) or keep
everything in `package main`.
**Decision:** All `.go` files in `package main`, split by concern using filename suffixes
(`handlers_books.go`, `handlers_patrons.go`, etc.).
**Rationale:** The app is medium-sized. Sub-packages would add import indirection without meaningful
benefit at this scale.

---

## DEC-004: Session cookies over JWT

**Date:** 2026-03-01 (CP2)
**Context:** Two common approaches for web app auth: server-side sessions with cookies, or
stateless JWT tokens.
**Decision:** Server-side sessions stored in the `sessions` DB table, with `HttpOnly`,
`SameSite=Strict` cookies. Token generated with `crypto/rand`.
**Rationale:** Server-side sessions can be invalidated immediately (logout, compromise). JWTs
cannot be revoked without maintaining a blocklist, which negates the stateless benefit. For a
server-rendered app with no separate API clients, sessions are simpler and more secure.

**Addendum (2026-04-10, CP4):** `SameSite=Strict` was documented as a decision here but the
original CP2 code never actually called `c.SetSameSite(http.SameSiteStrictMode)` before
`c.SetCookie`, so the browser used its default (Lax). Fixed while working on CSRF (#32) in CP4.
`HandleLoginPost` now explicitly sets Strict mode on both the session cookie and the
`csrf_login` pre-session cookie.

---

## DEC-005: bcrypt for password hashing

**Date:** 2026-03-01 (CP2)
**Context:** Password hashing options include bcrypt, scrypt, argon2.
**Decision:** Use `golang.org/x/crypto/bcrypt` with default cost factor.
**Rationale:** bcrypt is well-understood, widely supported, and the cost factor provides natural
brute-force resistance. Good enough for this application; argon2 would be overkill.

---

## DEC-006: Public kiosk with optional login

**Date:** 2026-03-13 (CP2 design update)
**Context:** Original design had the kiosk as an authenticated page. Reconsidered whether public
browsing should require login.
**Decision:** `GET /kiosk` is fully public (no auth required). Patrons can optionally log in to
access favorites and holds. Checkout and return remain staff-only on the book detail page.
**Rationale:** A kiosk in a real library should let anyone walk up and browse. Requiring login
defeats the purpose. Patron features (favorites, holds) are value-adds that justify optional login.

---

## DEC-007: SSE for real-time availability (not WebSocket)

**Date:** 2026-03-01 (deferred post-submission as of the 2026-04-19 CP6 v2 trim)
**Context:** Real-time availability updates need to push from server to browser.
**Decision:** Use Server-Sent Events (`GET /events`) instead of WebSocket when the feature is built.
**Rationale:** SSE is one-way (server to browser), which is exactly what this use case needs.
Simpler than WebSocket, works over HTTP/1.1, auto-reconnects, and requires no additional
dependencies.
**Status update 2026-04-19:** The SSE feature itself is deferred post-submission. CP6 ships
loans without live availability broadcast; a page reload is the signal. The technology choice
recorded here stands for whenever the feature is built.

---

## DEC-008: Open Library API via server-side proxy

**Date:** 2026-03-01 (planned for CP4)
**Context:** ISBN lookup needs to call the Open Library API to auto-fill book metadata.
**Decision:** Server-side proxy endpoint (`GET /api/openlibrary?isbn=...`) fetches data and
returns clean JSON to the client.
**Rationale:** Avoids CORS issues with direct browser requests. Keeps client JS simple. Allows
server-side ISBN format validation before making the outbound request.

---

## DEC-009: ZIP export/import with standard library

**Date:** 2026-03-01 (planned for CP7)
**Context:** Admin needs to export and import the full database and cover images.
**Decision:** Use Go's `archive/zip` standard library package.
**Rationale:** No third-party dependency needed. ZIP contains the SQLite file and cover images
from `data/covers/` (the DATA_DIR-relative path since cover storage moved in #20).

---

## DEC-010: WAL mode for SQLite

**Date:** 2026-03-01 (CP2)
**Context:** SQLite default journal mode blocks readers during writes.
**Decision:** Enable WAL mode on startup (`PRAGMA journal_mode=WAL`).
**Rationale:** Required by the product specification. WAL allows concurrent reads during writes,
which matters when SSE connections are reading while staff actions are writing.

---

## DEC-011: Pointer types for nullable database columns

**Date:** 2026-04-04 (CP3)
**Context:** Book fields like ISBN, publisher, description, and genre are optional (nullable in SQLite).
Scanning NULL into a Go `string` or `int` causes a runtime error.
**Decision:** Use pointer types (`*string`, `*int`) for nullable columns in Go structs. Register a
`deref` template function to safely unwrap pointers in templates.
**Rationale:** Pointer types make nullability explicit at the type level. The `deref` helper keeps
templates clean without requiring nil checks on every optional field.

---

## DEC-012: Client-side catalog filtering (temporary)

**Date:** 2026-04-04 (CP3)
**Context:** The catalog needs search, genre filtering, and availability filtering. Options: server-side
query params with page reloads, or client-side filtering over the full dataset.
**Decision:** All books are rendered server-side into the page. JavaScript in `app.js` filters by
toggling `display: none` on card elements using data attributes.
**Rationale:** Fastest path to a working catalog for CP3. This approach does not scale to larger
collections (thousands of books would hurt page load and DOM performance). Server-side pagination
and query-based filtering should replace this before production use.

---

## DEC-013: Template loading pattern

**Date:** 2026-02-01 (CP1), updated 2026-04-04 (CP3)
**Context:** Go templates can be loaded individually, as a global set, or as page-specific pairs.
**Decision:** `map[string]*template.Template` with one entry per page, each paired with
`layout.html`. The `renderTemplate()` helper executes the `"layout"` template which pulls in
the page's `"content"` block. As of CP3, templates are created with `template.New("layout").Funcs(funcMap)`
to register custom functions before parsing.
**Rationale:** Simple, explicit, and avoids the global template namespace collisions that come
from `template.ParseGlob()`. Each page knows exactly which templates it includes.

---

## DEC-014: Three-role access model (admin, staff, patron)

**Date:** 2026-04-06 (CP4)
**Context:** The original two-role model (admin/patron) had no way to give operational access
without full admin privileges. Different deployment environments need a middle tier for
day-to-day workers who can handle books, view patrons, and run exports, but should not
manage user accounts or system settings.
**Decision:** Three roles with chained middleware. `RequireAuth` validates the session and sets
the user in context. `RequireStaff` checks that the role is not patron (allows admin + staff).
`RequireAdmin` checks that the role is admin. Route groups chain these:
`RequireAuth, RequireStaff` for operational routes, `RequireAuth, RequireAdmin` for admin-only routes.
**Rationale:** Chaining keeps each middleware single-purpose and avoids duplicating session lookup
logic. The staff role fills the gap between full admin and patron without overcomplicating the
permission model.

---

## DEC-015: Admin page shared with role-based template conditionals

**Date:** 2026-04-06 (CP4)
**Context:** Both admin and staff need access to the admin tools page (export/import, system stats),
but admin has additional privileges (staff management, system settings).
**Decision:** Single `/admin` route in the staff group (accessible to admin + staff). The template
uses `{{if and .User (eq .User.Role "admin")}}` to conditionally show admin-only sections.
Admin-only POST endpoints are registered in the admin route group, not the staff group.
**Rationale:** Template conditionals control what the user sees; route group middleware enforces
what the user can do. This avoids duplicate pages and is consistent with the pattern used in the
sidebar, dashboard, and book detail templates. Security does not depend on the template -- even
if a staff user crafted a direct request to an admin-only endpoint, the middleware would return 403.

---

## DEC-016: Patron metadata as JSON TEXT column

**Date:** 2026-04-06 (CP5 design)
**Context:** Different deployment environments need different extra fields on patron records
(external IDs, housing units, departments, etc.). Options: EAV table (patron_custom_fields),
JSON column, or hardcoded extra columns.
**Decision:** Add a nullable `metadata TEXT` column to the patrons table. Stores JSON for
environment-specific fields. NULL for manually-added patrons. CSV imports store extra columns
as JSON in this field.
**Rationale:** JSON columns are the modern standard for flexible metadata. SQLite has built-in
JSON functions (`json_extract`) for querying. Avoids the EAV anti-pattern and keeps the schema
clean. If the project ever migrates to PostgreSQL, JSONB provides even richer support.

---

## DEC-017: Session-bound CSRF tokens with double-submit cookie for login

**Date:** 2026-04-10 (CP4)
**Context:** Every state-changing form needs CSRF protection. Several patterns exist:
double-submit cookie (non-HttpOnly cookie with a token, form embeds the same token, middleware
compares); synchronizer token bound to the server-side session (token stored on the session
row, template reads it, middleware validates); per-form tokens (new token on each render,
tracked in a pool).
**Decision:** **Session-bound synchronizer tokens** for authenticated routes, generated at
login and stored on the `sessions` row in a new `csrf_token NOT NULL` column. The token is
injected into the request context by `RequireAuth`/`LoadUser` alongside the user, then
auto-injected into template data by `renderTemplate` (same pattern as `User`), so forms
only need `<input type="hidden" name="csrf_token" value="{{.CSRFToken}}">`. A new
`CSRFProtect` middleware validates unsafe-method requests against the context token using
`subtle.ConstantTimeCompare`. For `POST /login` where no session exists yet, a separate
**pre-session double-submit cookie** named `csrf_login` is set on `GET /login` (HttpOnly,
SameSite=Strict, 10-minute lifetime) and compared against the form field by a dedicated
`LoginCSRFProtect` middleware. The pre-session cookie is cleared once a real session exists.
**Rationale:** Session-bound tokens are the textbook-correct pattern for server-rendered apps
with sessions and bind CSRF state to authentication state for free (logout deletes both).
Adding a column to `sessions` was a 1:1 extension with no extra joins. Per-form tokens would
add complexity (token pool, tracking, expiry) without meaningful security gain for LibreShelf's
threat model. The double-submit cookie for login is scoped narrowly to the authentication
chicken-and-egg and does not bleed into the normal request path. Defense in depth: the
SameSite=Strict cookie attribute (also implemented in CP4, see DEC-004 addendum) provides a
first line of defense before middleware ever runs.

---

## DEC-018: Canonical UTC datetime format for session expiry

**Date:** 2026-04-10 (CP4, discovered during CSRF integration testing)
**Context:** `CreateSession` originally passed `time.Time` directly to `dm.db.Exec`. The
modernc.org/sqlite driver serialized this as ISO 8601 with the local timezone offset and
nanosecond precision (e.g. `2026-04-10T22:22:05.168055574-06:00`). SQLite's `CURRENT_TIMESTAMP`
returns canonical UTC in `YYYY-MM-DD HH:MM:SS` format. The `GetSession` WHERE clause used
`expires_at > CURRENT_TIMESTAMP`, which is a **lexicographic string comparison** of two
differently-formatted strings. On non-UTC systems (e.g. local dev in MDT) the string comparison
failed even when the absolute times were correct, so freshly-created sessions were reported as
expired. The UTC EC2 deployment was accidentally masking this because the date portions matched.
Additionally, SQLite's `datetime()` normalization function could not parse the driver's
9-digit fractional seconds, so wrapping the comparison in `datetime()` returned NULL.
**Decision:** `CreateSession` formats `expires_at` explicitly as
`expiresAt.UTC().Format("2006-01-02 15:04:05")` before insert, producing a canonical UTC
string that is byte-identical in format to `CURRENT_TIMESTAMP`. String comparison then works
correctly as chronological comparison. `GetSession` keeps `datetime()` wrappers on both sides
of the comparison as defense in depth against future callers that might bypass `CreateSession`.
**Rationale:** Storing datetimes in a canonical format at write time is more robust than
normalizing at every read. Formatting in UTC removes the timezone-comparison trap entirely.
The `"2006-01-02 15:04:05"` layout is Go's reference time; it produces exactly the format
SQLite's `CURRENT_TIMESTAMP` returns. Discovered via `TestAuthenticatedPOSTWithCSRF`, which
was the first test to exercise the full `CreateSession` to `GetSession` round trip and caught
the latent bug immediately.

---

## DEC-019: Type-to-confirm over delayed deletion queue for staff accounts

**Date:** 2026-04-16 (CP5 design)
**Context:** Admin staff deletion needs a safety net against accidental or mistaken deletion.
Proposed design was a 48-hour delayed-deletion queue with cancellation. Evaluated against
LibreShelf's actual threat model (single-admin class deployment) and the engineering cost.
**Decision:** Use a type-to-confirm modal instead of a delayed queue. The delete button opens
a Bootstrap modal containing the target username. The confirm button stays disabled until
the admin types the username exactly. Deletion is immediate on confirm. No background job,
no pending state, no cancellation flow. Last-admin and self-deletion checks run server-side
and also gate the UI (delete button is disabled on rows that violate either rule).
**Rationale:** A 48-hour queue would require a new pending-deletions table, a startup
catch-up pass, a periodic scheduler goroutine, a cancellation UI, and a dual last-admin
check (at schedule time and execution time, because two admins could schedule each other
and leave zero admins). Each of those is a non-trivial lift and would need its own tests.
Type-to-confirm captures most of the "oops" protection value in one modal with roughly
fifty lines of client JS. Consistent with the Absolute Code philosophy: build the simpler
thing first; revisit if the threat model changes.

---

## DEC-020: Combined username + role edit, restricted role transitions

**Date:** 2026-04-16 (CP5 design)
**Context:** Staff management needs to support renames and role changes. Options: separate
endpoints per field (cleaner REST but more handlers), one combined edit endpoint, or
promote/demote as explicit actions.
**Decision:** One `POST /staff/:id/edit` endpoint updates both username and role in a
single call. Allowed role transitions are `staff <-> admin` only. Patron role changes are
out of scope and will be handled in `/patrons` (#21). Self role changes are forbidden
(cannot demote yourself, prevents accidental admin lockout). The last admin cannot be
demoted (same `CountAdmins() > 1` check used for deletion). The UI disables the `staff`
option in the role dropdown when the target is the last admin, and disables the role
field entirely when the target is the current user.
**Rationale:** Combining username and role into one form matches the UX of a standard
edit dialog and keeps the handler count low. Restricting role transitions to `staff <->
admin` is a consequence of the CP4 three-role model: the patron role has a `patron_id`
foreign key relationship and a different lifecycle, so co-mingling it with staff edits
would invite bugs. Server-side checks are authoritative; the UI restrictions are UX
polish only.

---

## DEC-021: Password complexity policy, enforced everywhere passwords are set

**Date:** 2026-04-17 (CP5)
**Context:** Until now the seed accounts used short, lowercase passwords (`admin123`,
`staff123`, `patron123`) and there was no policy on operator-chosen passwords for the
`ADMIN_PASSWORD` env override or future staff/patron create handlers. Staff management
(#39) needs to validate passwords at every entry point, not just at the handler layer,
or a seeded/override account can sidestep the rule.
**Decision:** Enforce a single `ValidatePassword` function in `validators.go` that
requires 8+ characters with at least one uppercase letter, one digit, and one special
character (anything `unicode.IsPunct` or `unicode.IsSymbol`). Seed passwords bumped to
`Admin123!`, `Staff123!`, `Patron123!`. `SeedDefaultUsers` calls `ValidatePassword` on
the resolved `ADMIN_PASSWORD` before hashing and `log.Fatalf`s on failure so an
invalid env override fails fast at startup. Every future password-setting path
(staff create, staff password reset, patron create, admin password change) must pass
through the same validator.
**Rationale:** Centralizing the rule in one function prevents drift. Fatal-on-invalid
for the env override is the right failure mode: silent-accept would let weak passwords
through, and warn-and-fallback would silently swap the admin password without notice.
Username format uses a separate `ValidateUsername` helper with lighter rules (3-32
chars, alphanumeric + underscore) because username complexity serves a different
purpose (collision avoidance, URL/log sanity) from password complexity.

---

## DEC-022: Multi-statement DB writes must run inside a transaction

**Date:** 2026-04-17 (CP5)
**Context:** CP5 introduces several operations that touch more than one table in a
single logical write: staff `DeleteUser` removes sessions + user rows, patron create
(#21) will insert into `patrons` + `users`, book create (#20) will insert into
`books` + `authors` + `book_authors`, CP6 checkout will touch `loans` +
`books.quantity_available`. The existing `SeedBooks` code did the book + author +
book_author inserts as independent `Exec` calls with no atomicity. A failure partway
through would leave a book with no authors, or partially-linked authors.
**Decision:** Any multi-statement DB write is wrapped in a `sql.Tx` using the
standard idiom:
```go
tx, err := dm.db.Begin()
if err != nil { return err }
defer tx.Rollback() // no-op after successful Commit
// ... tx.Exec / tx.QueryRow for every write ...
return tx.Commit()
```
`SeedBooks` was retrofitted into a loop that calls `seedOneBook(b seedBook) error`,
with the transaction scoped per book (so a failure on one book still seeds the
rest). `DeleteUser` already followed the pattern. Future multi-table writes in CP5
and CP6 (`CreatePatron` with its linked user, `CreateBook` with authors, checkout
and return, ZIP import) will use the same idiom.
**Rationale:** The `defer tx.Rollback()` + explicit `tx.Commit()` pattern is the
shortest correct way to handle transactions in Go: any early return rolls back
automatically, and a successful commit makes the deferred rollback a no-op. Partial
writes are the most common source of latent data-integrity bugs, and the cost of
wrapping in a transaction is a single extra function call per write. Adopting this
as a project standard now means every new handler in CP5-CP7 inherits the guarantee
by default.

---

## DEC-023: Variant B two-button submit for bulk-entry forms

**Date:** 2026-04-18 (CP5)
**Context:** After shipping book Create with a PRG redirect to `/books/:id`, it became
clear the single-destination flow hurt bulk-entry use cases (processing a donation
stack, manually importing a shelf of titles). Three options on the table: always
redirect back to `/books/new` after save, keep the single `/books/:id` redirect but
add a "View book" link in the success banner, or give the admin an explicit choice
at the moment of save with two submit buttons.
**Decision:** Two submit buttons on the create form. "Save" posts with
`submit_action=save` and redirects to `/books/:id` (detail page). "Save and Add
Another" posts with `submit_action=add_another` and redirects back to `/books/new`
with the form cleared. Flash cookies (`book_created` code plus the title in
`flash_detail`) fire on both paths so either redirect target shows
"Added to the catalog: **Title**" in the banner. The edit form shows only
"Save Changes" -- add-another is a create-only affordance. Any unexpected
`submit_action` value falls through to the default `/books/:id` redirect. The same
pattern will apply to Patrons (#21) where the bulk-entry vs single-add tension also
exists (a receptionist processing a stack of sign-up sheets vs. adding one patron
to immediately check their record).
**Rationale:** Django-admin pattern, battle-tested across years of CRUD UX. Gives
both workflows without sacrificing either, which the single-button alternatives
could not. The handler-side branch is one string comparison; the template-side
addition is one extra `<button>` gated on `{{if .ShowAddAnother}}`. Surface area is
small, the affordance is obvious to admins familiar with any other admin console,
and it generalizes cleanly to the next CRUD surface.

---

## DEC-024: Loan state expressed via `due_date + returned_at`, no status column

**Date:** 2026-04-20 (CP6 design session)
**Context:** CP6 introduces the loan lifecycle. A loan has three observable states
(active, returned, overdue). The naive approach is a `status` TEXT column updated
on checkout and return, but that denormalizes data already expressible from two
timestamp columns and creates drift risk: a row could read `status='active'` while
`due_date` is in the past, or `status='returned'` while `returned_at IS NULL`.
**Decision:** The `loans` table carries `due_date DATE NOT NULL` and
`returned_at DATETIME` (nullable). No `status` column. The three states are derived
at query time:
- **Active:** `returned_at IS NULL AND due_date >= DATE('now')`
- **Overdue:** `returned_at IS NULL AND due_date < DATE('now')`
- **Returned:** `returned_at IS NOT NULL`

`due_date` is a DATE (not DATETIME) because loan terms are whole-day granularity;
a book due "April 30" is due at end-of-day April 30, and fractional hours never
matter. `returned_at` stays DATETIME because the exact return moment is useful in
audit history. The existing `loans` table schema from CP1 (DATETIME `due_date`
nullable, no `fine_cents`) is rewritten in `createSchema`; local dev re-seeds via
`rm data/database.sqlite*` and EC2 gets a clean DB at CP7 deploy.
**Rationale:** One source of truth per state. No branch where a handler might
forget to update `status` and leave the row inconsistent. The `DATE('now')`
comparison is trivial in SQL and readable in both queries and tests. If a future
CP adds a fourth state (lost, on-hold, etc.) that is genuinely not derivable from
existing columns, adding a status column then costs one migration -- cheap.

---

## DEC-025: Loan management via `/loans` with filter query param, 14-day loan term

**Date:** 2026-04-20 (CP6 design session)
**Context:** CP6 needs a staff-facing way to see and act on loans. The v1 planning
doc proposed a rapid-scan checkout/checkin portal, a separate `/reports/overdue`
table, and per-patron printable notices. The v2 reality-check trimmed all of that
to fit the calendar. The minimum shippable surface is: one list view that staff
can filter between active and overdue loans, with a return action per row, plus
checkout initiated from the book-detail page's existing scaffold. Loan term
default needed a number.
**Decision:** Single `/loans` route, staff + admin only, with
`?filter=active|overdue` query param (default `active`). Table rows show book
title, patron name, due date, and (when overdue) days overdue; each row has a
Return button posting to `POST /loans/:id/return`. Default loan term is 14 days,
defined as a single package-level constant (`const DefaultLoanTermDays = 14`)
passed to `CheckoutBook` by the handler. Per-book or per-patron overrides deferred
post-submission. The book-detail page keeps its existing checkout scaffold as the
checkout UX; no separate `/checkout` portal in CP6. The rapid-scan portal design
is preserved in bead `cs408-go-stack-yu3` for a future un-defer when transaction
volume justifies it.
**Rationale:** Two states expressible by one query param keeps the URL shareable,
the template single, and the tests trivial. Fourteen days is the standard public
library default (Boise Public, Ball State Bracken, most US library systems) and
shorter terms produce more overdue rows for demo data. Keeping the book-detail
form as the checkout UX means session 3 is "wire existing scaffold to handler"
instead of "build new portal template" -- ~2h saved.

---

## DEC-026: Checkout guardrails: block on any overdue, cap active loans at 5

**Date:** 2026-04-20 (CP6 design session)
**Context:** Checkout can be unrestricted (trust staff) or guarded. Unrestricted
is simpler but lets a patron with six overdue books walk out with a seventh.
Guardrails are the stronger library posture. Two guardrails on the table: block
checkout when the patron has overdue loans, and/or cap the number of concurrent
active loans per patron.
**Decision:** Both guardrails in CP6. Enforced inside `CheckoutBook` before the
insert, as part of the same transaction that decrements `quantity_available`:

1. **Block on overdue:** if the patron has one or more loans where
   `returned_at IS NULL AND due_date < DATE('now')`, refuse the checkout and
   return `ErrPatronHasOverdue`.
2. **Max active loans:** if the patron has 5 or more loans where
   `returned_at IS NULL`, refuse the checkout and return `ErrPatronAtLoanLimit`.

The limit is a single package-level constant (`const MaxActiveLoansPerPatron = 5`).
Handlers map both sentinels to flash codes (`loan_blocked_overdue`,
`loan_blocked_limit`) surfaced as a banner on the book-detail page.
**Rationale:** "Any overdue blocks" is the simplest rule to explain to both staff
and patrons, and matches typical small-library practice (return what you have,
then borrow more). Five active loans per patron is the sweet spot between too
restrictive (3 feels stingy for families) and too liberal (10 makes the dashboard
"Active Loans" counter less meaningful). Both numbers are single constants, so
tuning them later is one-line change. Catching these inside `CheckoutBook` keeps
the check and the insert atomic; validating in the handler would open a TOCTOU
race between check and insert when multiple staff check out books concurrently.

---

## DEC-027: Admin backup/restore -- ZIP scope, consistency, swap strategy, safety interlocks

**Date:** 2026-04-26 (CP7 design session)
**Context:** CP7 ships an admin panel feature (#23) that lets an admin export
the library to a portable backup ZIP and import a backup ZIP to restore state.
The export must produce a consistent point-in-time copy under concurrent
writes; the import must not silently corrupt operator data, must reject
malicious archives (Zip Slip), and must be reversible if the operator picks
the wrong file. Seven sub-decisions, captured together because they are
intertwined.

**Decision:**

1. **ZIP scope.** Backup contains exactly two paths: `data/database.sqlite`
   (the entire DB including the sessions table) and the `data/covers/`
   directory. Logs, config, the binary, templates, static assets, and
   `.beads/` are explicitly out of scope -- they come from the deploy and
   from git, not from operator data.

2. **Export consistency.** Use SQLite's `VACUUM INTO '/tmp/snapshot.sqlite'`
   to produce a consistent point-in-time copy before zipping. ZIP the
   snapshot file (not the live DB), then delete the snapshot. This rules
   out torn-write corruption when a checkout/return happens during export.

3. **Import strategy.** In-process swap with a global `sync.RWMutex`. The
   import handler takes the write lock, closes `*sql.DB`, swaps files,
   reopens. All other handlers take the read lock when accessing the DB.
   No restart required. Lock window is ~200-500ms on a small DB; invisible
   to a single-library deployment with ~5 concurrent users.

4. **Pre-swap backup.** Before overwriting, rename existing files
   to `data/database.sqlite.bak` and `data/covers.bak/` (atomic on the
   same filesystem). On reopen failure, rename `.bak` back and reopen the
   original. On reopen success, keep the most recent `.bak` indefinitely
   (next import overwrites it). Exclude `*.bak` and `*.bak/` from the
   export walk so backups do not recursively contain previous backups.

5. **Zip Slip protection.** Carved into a new `internal/safezip/` package
   (first use of `internal/` in this repo). `SafeExtract(zipPath, destDir)`
   validates every entry: `filepath.Rel(absDest, targetPath)` must not
   start with `..` and must not be absolute; entries with the symlink mode
   bit are rejected outright. Unblocks future reuse by the deferred `#63`
   offline cover sync workflow without a refactor.

6. **No CLI counterpart in CP7.** Web UI export only. A CLI subcommand
   (`go run . export-backup --out=...`) was considered for SSH/cron
   workflows and for shared plumbing with `#63`'s `cmd/cover-fetcher/`,
   but deferred to keep CP7 scope tight. Operators who need scripted
   backups can `curl -b cookies.txt /admin/export -o backup.zip` against
   the same handler.

7. **Import confirmation interlock.** Medium friction: Bootstrap modal +
   a checkbox labeled "I have a backup. I understand this replaces the
   current database." The Confirm button is disabled until the box is
   ticked. Lighter (modal-only) was rejected because users blow past
   reflexive Confirm dialogs; heavier (type the word "REPLACE") was
   rejected because the `.bak` rollback (decision 4) gives a recovery
   path even if the operator clicks through.

**Rationale:** Each of the seven was a balance between security posture
and complexity tax. The pattern is: pick the safer option whenever the
implementation cost is small. `VACUUM INTO` is one statement; safezip is
a small package; pre-swap rename is two `os.Rename` calls. The cumulative
result is an admin panel where the destructive operation has three
independent layers of protection (Zip Slip rejection, write-lock atomic
swap, `.bak` rollback) and the operator has to deliberately tick a box
to fire it. The choice to skip the CLI and to pick medium-not-heavy
confirmation explicitly trades small marginal protection for keeping
CP7 scope ship-able by the 2026-04-30 target. The `internal/safezip/`
package extraction is the one place where we paid future-tax now (against
the "don't design for hypothetical future requirements" rule) because
`#63`'s design explicitly depends on it and the alternative is a refactor
on the day we need it.

---

## DEC-028: Defensive HTTP headers + trusted-proxy pin + Go 1.25.9 stdlib bump

**Date:** 2026-05-01 (CP7 #24 implementation)
**Context:** Default Gin trusts every proxy for `X-Forwarded-For`, the bare
binary sends no defensive HTTP headers, and `govulncheck` on the Go
1.25.0 toolchain reported 19 stdlib CVEs (net/url, encoding/pem,
crypto/tls, crypto/x509, etc.). All three are zero-cost to fix together
before the final EC2 deploy.

**Decision:**

1. **`SecurityHeaders` middleware applied router-wide before everything
   else.** Sets `X-Content-Type-Options: nosniff`, `X-Frame-Options:
   DENY`, `Referrer-Policy: same-origin`, and a Content-Security-Policy
   of `default-src 'self'; style-src 'self' 'unsafe-inline'; img-src
   'self' data:; frame-ancestors 'none'; base-uri 'self'; form-action
   'self'`. Applied via `router.Use(SecurityHeaders)` so even 404 / 500
   responses carry the headers. HSTS is gated on `APP_ENV=production`
   because the bare-IP EC2 deploy is HTTP-only and HSTS over HTTP is
   harmful.

2. **`'unsafe-inline'` allowed for `style-src` only, not `script-src`.**
   Templates rely on inline `style="..."` attributes in 22+ places, and
   tightening that without breaking the visual design is a multi-day
   refactor. The single inline `<script>` block in `backup_admin.html`
   was extracted to `static/javascripts/admin_backup.js` so `script-src`
   can stay tight at `'self'` (no inline, no eval). XSS protection
   therefore lives in `script-src`; `style-src 'unsafe-inline'` is the
   accepted residual risk.

3. **`router.SetTrustedProxies([]string{"127.0.0.1"})` in `main.go`.**
   On the EC2 deploy, nginx fronts the Go app on localhost:3000, so
   only nginx should be allowed to set `X-Forwarded-For`. Default Gin
   trust-everyone behavior would let any client spoof their IP. The
   test router in `setupTestRouter` mirrors this so the test surface
   matches production (per the #35 gotcha).

4. **Pin Go 1.25.9 in `.tool-versions` and `go.mod`.** `govulncheck`
   on 1.25.0 reported CVEs all fixed in 1.25.x patch releases. 1.25.9
   was the latest available via asdf at deploy time. Re-running
   `govulncheck` on 1.25.9 returns no findings. Pinning in
   `.tool-versions` makes `asdf install` reproducible; bumping the
   `go` directive in `go.mod` makes anyone running a stale toolchain
   fail at compile time rather than silently shipping vulnerable
   binaries.

5. **`govulncheck` and `go mod verify` are pre-deploy gates, not CI
   gates.** Documented in `docs/deployment.md` Step 1 and
   Redeploying section. CP7 did not set up GitHub Actions CI (#17
   remains open); the gates run locally on the developer machine
   before each `scp`. If/when #17 lands, the gates move into the CI
   workflow.

**Rationale:** Each of the four was a few lines of code (or zero, for
the toolchain pin) with high security upside, and they cluster naturally
into one PR (#75). The decision NOT to refactor inline styles is the
load-bearing trade -- it keeps CSP headers shippable today instead of
deferring all of them indefinitely waiting on a perfect template
refactor. The `script-src 'self'` constraint required moving one
small JS file, which surfaced as a nice forcing function: any future
inline script will fail loudly via a CSP violation in DevTools console
rather than slip in unnoticed.

---

## DEC-029: Admin tools-index pattern (heavy tools as cards, light settings inline)

**Date:** 2026-05-01 (CP7 mid-implementation, after DEC-027 backup
shipped)
**Context:** DEC-027 shipped `/admin/backup` as a dedicated page,
leaving `/admin` as the placeholder "Admin panel coming soon" stub.
With more admin features likely to land post-CP7 (patron self-
registration toggle `cs408-go-stack-o1x`, offline cover sync
`cs408-go-stack-0eh`), the question was whether to fold backup into
`/admin` (one big page) or build out `/admin` as a tools dashboard.

**Decision:**

1. **`/admin` is a tools-index page**, not a stub and not a single
   monolithic admin form. The page has labeled sections; each section
   is either a card grid linking out to a dedicated tool page, or an
   inline block of lightweight controls.

2. **Heavy tools get their own card on `/admin` and drill into a
   dedicated `/admin/<tool>` page.** "Heavy" means: multi-section
   layout, modal interlocks, file uploads, multi-step flows, or
   substantial per-tool state. Backup is the canonical heavy tool --
   stats panel + export + import-with-modal -- and it lives at
   `/admin/backup`.

3. **Light tools live inline as sections directly on `/admin`.** "Light"
   means: a single toggle, a single text field, a single button. The
   patron self-registration toggle (when it ships) is the canonical
   light example: one boolean setting, one form, one button. No need
   for a dedicated `/admin/registration` route.

4. **`/admin` moved from staff-accessible to admin-only access.** The
   pre-CP7 placeholder was in the staff route group, but every actual
   admin tool is admin-only. The mismatch (staff sees an Admin link to
   a page with no tools they can use) is bad UX. Mirrored in
   `setupTestRouter`; `TestStaffRoleCannotAccessAdminRoutes` was
   updated to assert the new boundary.

5. **The standalone "Backup" link in the sidebar was removed.** Only
   the "Admin" link remains under the admin-only block. Drilling from
   `/admin` -> Backup card -> `/admin/backup` is the path. New tools
   add a card on `/admin`, not a sidebar entry.

**Rationale:** Naive future-proofing said "build a sidebar group with
Admin > Backup > Future Thing". That visual nesting was overkill for
the actual backlog (probably 3-4 admin tools total over the project
lifetime). The tools-index pattern scales linearly without sidebar
clutter and makes the access-tier boundary explicit at the route
level, not just the template level. The card-vs-inline split is a
judgment call per tool; the criterion ("does this need its own
page?") is loose on purpose so future tools don't get force-fit
into the wrong category.
