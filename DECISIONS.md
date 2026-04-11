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

**Date:** 2026-03-01 (planned for CP6)
**Context:** Real-time availability updates need to push from server to browser.
**Decision:** Use Server-Sent Events (`GET /events`) instead of WebSocket.
**Rationale:** SSE is one-way (server to browser), which is exactly what this use case needs.
Simpler than WebSocket, works over HTTP/1.1, auto-reconnects, and requires no additional
dependencies.

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
from `static/images/covers/`.

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
