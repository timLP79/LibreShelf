# LibreShelf -- Claude Code Instructions

This file is read automatically at the start of every session. It contains standing context and
working agreements for this project.

---

## About This Project

LibreShelf is a self-hostable library management system built for CS408 Spring 2026 at Ball State.
It lets a small library manage books, patrons, and loans through a simple web UI. A public kiosk
supports self-service browsing with optional patron login for favorites and holds. All checkout and
return transactions are staff-only.

**Live at:** EC2 instance (URL available on request)
**Repo:** github.com/timLP79/cs408-go-stack

---

## How We Work Together

- We are a team. Discuss approach before building anything non-trivial.
- Before starting a new feature or component, talk through the design, security implications,
  best practices, and future-proofing. Make the decision together, then build it right.
- Do not silently skip best practices. If something should be done, raise it before writing
  code, not after.
- Keep solutions simple and practical. Do not over-engineer. But do build things correctly
  from the start.
- No em dashes in any written output.
- No co-author notes in commits, code, or documentation.
- Direct and honest assessments over validation.
- Use feature branches for substantial changes that could break functionality. Small fixes
  (typos, one-liner bug fixes) can go straight to main. Otherwise, create a feature branch,
  test, and merge via PR.

---

## Coding Collaboration

**Go (this project):** Tutor mode. Show what needs to be written and explain it thoroughly.
Do not use Write/Edit tools to create or modify Go files. Tim writes all Go code.
**Exceptions:** Claude can directly edit SQL schema in `createSchema()`, repetitive data entry
(seed data, struct literals), and test files.

**HTML templates, CSS, JS:** Claude can write and edit these files directly.

**School coding (Java, C, Python for class):** Tutoring mode. Guide, do not generate.

**Work coding (Snowflake SQL, Apps Script, Streamlit):** Generate and explain fully.

---

## Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.24+ |
| Web framework | Gin (`github.com/gin-gonic/gin`) |
| Templating | Go `html/template` with layout pattern |
| Database | SQLite via `modernc.org/sqlite` (pure Go, no CGo) |
| CSS | Bootstrap 5.3 (served locally, no CDN) |
| Deployment | EC2 + systemd + nginx |

---

## Commands

```bash
go run .                # start the app on :3000 (PORT env to override)
go build -o go-full-stack .
go test ./...           # full suite (35 passing on cp5-crud)
go test -v -run TestX   # run a specific test
sqlite3 data/database.sqlite  # inspect the local DB
```

Deploy guide: `docs/week6/deployment.md` (build, scp, systemctl).

---

## Dev Environments

- **Laptop:** Ubuntu 24.04
- **Desktop:** Fedora Kinoite host, distrobox Ubuntu 24.04 container

---

## Infrastructure

- Deployed to EC2 with systemd service (`deploy/go-full-stack.service`)
- Reverse proxy: nginx
- Secrets: environment variables (PORT, DATA_DIR, DB_NAME, ADMIN_PASSWORD)
- Database file: `data/database.sqlite` (gitignored)
- HTTPS: not available on bare IP deployment; HTTP-only is acceptable for class

---

## Current State

**CP1 -- Project Skeleton:** Complete. All routes, nav, schema, basic tests.
**CP2 -- Authentication:** Complete. Login/logout, sessions, bcrypt, role-based access control.
**CP3 -- Book Catalog & Detail Pages:** Complete. Catalog with search/filter, book detail with metadata and loan history, bug fixes #28/#29/#30, responsive sidebar.
**CP4 -- Security Hardening + Three-Role Model:** Complete. Three-role model, ExecuteTemplate buffer-based rendering, constant-time login, session-bound CSRF protection with pre-session double-submit cookie for login, SameSite=Strict on session cookie, canonical UTC datetime format for session expiry. 15 tests passing.
**CP5 -- Staff Management (#39):** Complete. `handlers_staff.go` covers list/create/edit/delete/reset-password with IDOR, self-demote/delete, and last-admin guards. Flash-cookie-based PRG messaging via `flash.go` (HttpOnly + SameSite=Strict, error-code slugs mapped to banner text server-side). Bootstrap inline live validation across all three modals (per-field `is-invalid`/`is-valid` as the user types, `novalidate` on forms). `UpdateUserPassword` DB method atomically wipes target sessions (DEC-022). 20 new handler tests plus admin-group route boundary coverage.
**CP5 -- Book CRUD (#20):** Complete. `handlers_books.go` covers Create / Edit / Update / Delete plus the `/api/openlibrary/isbn/:isbn` proxy. Shared `templates/book_form.html` renders both new and edit with Variant B two-button submit (DEC-023). `openlibrary.go` is the server-side OL client (DEC-008); `covers.go` validates uploads by size, extension, and magic-byte MIME and stores under `DATA_DIR/covers/`. Cover replacement on edit deletes the old file on disk after the DB write succeeds; delete cleans up the cover too. Duplicate-ISBN guard excludes the book being edited from the conflict check. `ErrBookHasLoans` sentinel blocks delete when loans exist (surfaces as a form banner, not a 500). Flash system gains a companion `flash_detail` cookie so the banner reads "Added to the catalog: **Title**" on either PRG destination. 28 new handler tests plus boundary coverage for the new routes.
**CP5 -- Patron CRUD (#21):** Complete. `handlers_patrons.go` covers list / create / edit / delete. `CreatePatron` is transactional (patrons + users per DEC-022) with auto-generated username via `generateBaseUsername` (first initial + last word, lowercased, non-alphanumerics stripped) and a collision-retry loop inside the transaction that suffixes `2`, `3`, ... until `SELECT COUNT FROM users WHERE username = ? COLLATE NOCASE` returns zero. `DeletePatron` is transactional (sessions + users + patrons) with the `ErrPatronHasLoans` guard mirroring the book pattern. Username is not editable once assigned (rename via delete-recreate). Patron reset-password and metadata UI were cut during rescope on 2026-04-18 to keep CP5 shippable; both tracked in the post-submission backlog. Catalog polish pass also landed: 4-wide grid on desktop, letterboxed covers against near-black slot with aspect preserved. 93 tests passing on `cp5-crud` -- 13 new patron-handler tests, 12 `generateBaseUsername` subtests.

Files that exist:
- `main.go`, `db.go`, `handlers.go`, `handlers_auth.go`, `handlers_books.go`, `handlers_patrons.go`, `handlers_staff.go`, `openlibrary.go`, `covers.go`, `flash.go`, `validators.go`, `main_test.go`, `handlers_books_test.go`, `handlers_patrons_test.go`
- All HTML templates including `staff.html`, `patrons.html`, and `book_form.html` (on branch `cp5-crud`), layout with responsive offcanvas sidebar
- `static/javascripts/app.js` (catalog filtering, staff/patron/book modals, OL Lookup wiring, Bootstrap live validation)
- `static/stylesheets/style.css` (custom styles including availability badges, responsive sidebar, 4-wide catalog grid, letterboxed cover slots)
- `static/images/favicon.svg` (custom bookshelf icon)

---

## Standards

- Handle all errors explicitly in Go -- never ignore returned errors
- Log errors server-side, return generic messages to clients
- Use environment variables for all secrets -- never hardcode
- Return correct HTTP status codes
- Validate and sanitize inputs server-side on every endpoint
- Rate limiting and CORS are CP7 scope. When you add a new endpoint before CP7, note the gap rather than assuming middleware exists.
- Always use parameterized queries (`?` placeholders) -- never string concatenation
- Commits should be descriptive and reference issue numbers where applicable
- Keep solutions lightweight -- consistent with the Absolute Code philosophy

---

## Gotchas

- **SQLite driver name.** `modernc.org/sqlite` registers as `"sqlite"`, not `"sqlite3"`. Don't copy-paste snippets from `mattn/go-sqlite3` docs (DEC-002).
- **Seed passwords are fresh-install-only.** `SeedDefaultUsers` skips users that already exist. Bumping a seed value does NOT update existing rows; `rm data/database.sqlite*` to re-seed locally.
- **Test router uses the production middleware chain** (fixed in #35). `setupTestRouter` returns `(router, dm)` and mirrors `main.go` route groups exactly. Use `loginAs(t, dm, username, role)` to get a session cookie + CSRF token, then `req.AddCookie(sess)` and set `csrf_token` on POSTs. `logoutHelper` exists for the logout path.
- **Schema changes don't migrate.** `createSchema` uses `CREATE TABLE IF NOT EXISTS`. Altering a column requires either `ALTER TABLE` or nuking `data/database.sqlite` locally.
- **Go has no hot-reload.** Template edits take effect only after a process restart (templates are parsed once at startup via `template.Must` in `main.go`). Go source edits take effect only after re-running `go run .`. Symptom of forgetting: the browser sees the old behavior with no errors. If something "didn't do anything," restart the server first.
- **Static assets cache aggressively in the browser.** `router.Static` serves `static/javascripts/app.js` and `static/stylesheets/style.css` without cache-busting query strings or asset fingerprinting, so after a JS/CSS edit the browser may still hold the prior version. Symptom: `typeof initWhateverIJustAdded` is `"undefined"` in the console even though the file on disk is current. Fix during dev: hard refresh (Ctrl+Shift+R) or keep DevTools open with "Disable cache" checked in the Network tab. Proper fix is CP7 polish -- a `?v=<build-time>` query or content-hash URL.

---

## Key References

- [Technical plan and architecture](./docs/plan.md) -- single source of truth for design
- [Product specification](./docs/week7/LibreShelf%20-%20Product%20Specification.pdf)
- [UI wireframes](./docs/week7/wire-frames/)
- [Security plan](./docs/security.md)
- [Deployment guide](./docs/week6/deployment.md)
- [Design decisions log](./DECISIONS.md)

---

## Open Issues / Current Focus

**Deadline: 2026-05-01.** Scope rescoped on 2026-04-18 to fit the calendar. CP boundaries preserved; a few items moved between checkpoints and a few sub-features deferred post-submission (see bottom).

**Target close dates:** CP5 by 4/24, CP6 by 4/27, CP7 by 4/30. Buffer day 5/1.

### CP1-CP4 (complete)

See `Current State` above for the per-CP summary. Closed issues and scope live in `docs/plan.md` and `git log`.

### CP5 -- CRUD Features (Staff, Books, Patrons) -- complete on `cp5-crud`

All four CP5 issues closed (landed 6 days ahead of the 4/24 target).

- [x] #35 -- Fix: Test router does not mirror production middleware (closed in b9ce9ed). `setupTestRouter` mirrors main.go route groups. Added `loginAs` and `logoutHelper` test helpers. Three new regression-pin tests cover the middleware chain.
- [x] #39 -- Staff management: closed. `handlers_staff.go` holds list/create/edit/delete/reset-password handlers. Admin-only route group registered in `main.go`. Flash-cookie messaging (`flash.go`) replaces URL query-param error surface; codes map to banner text server-side so operator text never transits the cookie jar. `UpdateUserPassword` is transactional and wipes target sessions on every reset. Bootstrap live validation across Add/Edit/Reset modals. 20 new handler tests plus three admin-group boundary tests in `main_test.go`.
- [x] #20 -- Book CRUD + Open Library API: closed. `handlers_books.go` holds Create / Edit / Update / Delete plus the `/api/openlibrary/isbn/:isbn` JSON proxy. Routes split across staff group (create + edit) and admin group (delete) per #20 design. `openlibrary.go` (DEC-008) + `covers.go` (upload validation, DATA_DIR/covers storage) + `flash.go` (detail cookie companion) back the handlers. Shared `book_form.html` template with Variant B two-button submit (DEC-023). Cover routing on update: upload > OL URL > preserve existing, with old-file cleanup after successful `UpdateBook`. Delete runs the `ErrBookHasLoans` guard and removes the cover file best-effort. 28 new handler tests in `handlers_books_test.go` plus `/books/new` and `/books/:id/edit` added to the auth/role boundary loops in `main_test.go`.
- [x] #21 -- Patron CRUD (CSV import, reset-password, and metadata UI deferred post-submission): closed. `handlers_patrons.go` holds list / create / edit / delete in the staff+admin route group. `CreatePatron` is transactional (patrons + users per DEC-022) with `generateBaseUsername` (first initial + last word, non-alphanumerics stripped) feeding a collision-retry loop inside the tx that suffixes `2`, `3`, ... until `SELECT COUNT(*) FROM users WHERE username = ? COLLATE NOCASE` returns zero. `DeletePatron` is transactional (sessions + users + patrons) with `ErrPatronHasLoans` guard mirroring the book pattern. Admin-typed temp password at create time (Variant 1). Username not editable once assigned. `patrons.html` modeled on `staff.html` with Add / Edit / Delete modals (no Reset); `initPatronManagement` in `app.js` clones `initStaffManagement` patterns. 13 new handler tests in `handlers_patrons_test.go` plus 12 `generateBaseUsername` subtests in `validators_test.go`. Catalog polish pass (4-wide grid, letterboxed covers) landed alongside #21 as late-session UI iterations.

### CP6 -- Loans + Kiosk + Pagination

Scope disciplined via the v2 reality-check on 2026-04-19: CP6 ships the foundation that makes the software function. Workflow polish (rapid-scan portal, sidebar restructure, dashboard redesign, printed overdue notices, patron-facing mini-lists) moves to the post-submission backlog. Full plan and deferred-design notes in `docs/cp6-planning.md`.

- [ ] **#22 -- Loan system (trimmed):** loan schema (`due_date`, `returned_at`, `fine_cents` future hook) + transactional DB methods + checkout/return handlers wired to the existing book-detail scaffold + `/loans` page with `active | overdue` query-param filter + kiosk public anonymous browse. Favorites only if time permits. SSE live availability and patron holds deferred post-submission (unchanged from CLAUDE.md original).
- [ ] **#37 -- Server-side pagination and filtering for catalog.** Needed now that the catalog seeds to 100 books on a fresh DB.
- [ ] **Dashboard placeholder wire-up (no redesign).** Replace the three "—" stubs in `templates/index.html` with real `SELECT COUNT(...)` queries. No card restructuring, no role-differentiated variant, no Overdue/Today's Activity/Out-of-Stock cards in CP6.
- [ ] **DEC-024, DEC-025, DEC-026 written in session 2** (loan state model, `/loans` list view, `fine_cents` reservation). Other design DECs around portal/sidebar/dashboard/notices deferred with the features themselves.

### CP7 -- Admin Panel + Security Hardening + Deploy

- [ ] #23 -- Admin panel: ZIP export and import (with Zip Slip protection)
- [ ] #24 -- Testing, polish, and deploy
    - `SecurityHeaders` middleware, `SetTrustedProxies`, `go mod verify`, `govulncheck`, final EC2 redeploy with a clean DB to pick up new seed passwords.

### Deferred post-submission backlog

- CSV patron import (from #21)
- Patron reset-password handler + modal (cut from #21 to keep CP5 shippable on 4/24): mirrors the staff reset-password pattern (Variant 1 admin-typed). Today's recovery path is "admin deletes the patron and recreates with a new password", which is acceptable for a small library but not great UX. Straightforward add when we have the time: one POST handler + one modal + ~3 tests. Variant 2 (server-generated temp + force-change) is the stronger long-term path, tracked separately in the staff Variant 2 entry below.
- Patron metadata column UI (cut from #21): `patrons.metadata` is a JSON TEXT column per DEC-016, left null by #21's create flow. Future work could add a free-form notes textarea or structured fields (student ID, library card number) for libraries that want to track more than name/email/phone.
- Patron activate / deactivate (raised during #21 smoke test, deferred post-submission): a soft-deactivation flag on the patron record so admins can suspend a patron temporarily without destroying the row (useful when the patron has active loans blocking delete, or leaves the library for a stretch and may return). Estimated ~1.5-2h: add `patrons.is_active BOOLEAN DEFAULT 1` (requires fresh DB since CREATE TABLE IF NOT EXISTS cannot ALTER), activate / deactivate handler pair (mirror staff reset-password shape), login middleware branch that rejects authentication for users whose linked patron is inactive, session wipe on deactivate (same idiom as `UpdateUserPassword` per DEC-022), Actions-column button that toggles state, and an "inactive" badge on the patron row. Maps cleanly onto the existing flash system (`patron_activated`, `patron_deactivated` codes).
- SSE live availability updates (from #22)
- Patron holds on checked-out books (from #22)
- Staff table responsive polish (from #39): "Reset Password" label wraps mid-word on narrow viewports; raw ISO-8601 timestamps (`2026-04-18T04:54:01Z`) break awkwardly; Actions column buttons get cramped below `md`. Options: friendly date format (server-side or via template helper), icon-only action buttons with tooltips, or stacked-card layout below `md`. Same treatment will be needed for patron and book tables once CP5/CP6 land, so solve it once and reuse.
- Password-reset Variant 2 (from #39): server-generated temporary password + force-change-on-next-login flow. Requires a `must_change_password` column on `users`, login-middleware branch that redirects flagged users to a `/change-password` page, a self-service password-change handler + template, and a one-shot display of the generated temp password. Variant 1 (admin-chosen password, current implementation) is acceptable for a small trusted staff; Variant 2 is the stronger posture where the admin never learns the user's long-term password.
- Orphan cover cleanup on post-cover-save validation failure (from #20): `HandleBookCreate` and `HandleBookUpdate` save the cover to disk BEFORE the duplicate-ISBN pre-check and DB write. If a cover-save succeeds but a later step fails (duplicate ISBN, transient DB error), the file is orphaned under `data/covers/`. Low-frequency leak in practice since duplicate ISBNs are caught client-side before submit in the common case, but should eventually be fixed by either (a) staging the cover bytes in memory until the DB write succeeds, or (b) a janitor pass that sweeps `data/covers/` for files not referenced in `books.cover_filename`. Same issue exists in the Create flow shipped in #20 and in the Update cover-replacement flow.
- Offline detection for the Open Library Lookup button (from #20): when the user's browser can't reach OL, hide or disable the Lookup button rather than leaving it to always-fails-until-timeout. Plan is **A + D combined**: (A) `navigator.onLine` + `online` / `offline` event listeners handle the unplugged-laptop and airplane-mode cases synchronously and for free; (D) on the first OL fetch failure, set a session-scoped flag that hides the Lookup button for subsequent form opens in the same session and surfaces "Open Library unreachable. Fill in manually." inline. All in `app.js`, no server changes, no probe endpoint. Known gap: a network that has general internet but blocks `openlibrary.org` still shows the button on first attempt -- acceptable since D catches it on click with a clear message. If we later want stronger "actually reachable from EC2" detection, layer on a server-side probe at startup (or cached per-N-minutes) that renders the form with a `OpenLibraryReachable` template flag.
- [ ] #17 -- Automate deployment via GitHub Actions (already low-priority backlog)
- **Checkout/checkin portal (rapid-scan, from CP6 v2 trim on 2026-04-19):** dedicated `/checkout` and `/checkin` staff pages, patron-selected-once + ISBN-rapid-entry flow, scanner-compatible (barcode scanners emit keyboard + Enter). Variant B design preserved in `docs/cp6-planning.md`. Earns its place at volume; a 10-50 transactions/day library works fine on the per-book book-detail flow shipped in CP6.
- **Sidebar restructure (from CP6 v2 trim):** current sidebar has "Admin" and "Staff" as confused peers. Proposed fix moves section headers to describe domain-of-work (Navigation / Management / Administration, or a four-section variant with Circulation) rather than access tier, with role gating per-item. Three variants documented in `docs/cp6-planning.md`. `/admin` route retained but relabeled to "Data Tools" when the restructure lands.
- **Dashboard redesign with role-differentiated cards (from CP6 v2 trim):** separate patron view (My Active Loans mini-list, My Favorites, My Holds placeholder, Out of Stock) vs staff/admin view (Overdue badge, Today's Activity, Active Loans, Out of Stock). Design principle: every card answers "is anything urgent," "what's happening now," or "where do I go to act." Full card set and mockups in `docs/cp6-planning.md`.
- **Overdue notice print system (from CP6 v2 trim):** `/reports/overdue` table with per-loan row granularity, click-through to per-patron printable notice, browser `@media print` + `@page` CSS (no server-side PDF). CP6 ships the "know what's overdue" half via the `/loans?filter=overdue` filter; the formatted mailable notice is the deferred polish. Full design including SQL queries in `docs/cp6-planning.md`.
- **Patron-facing dashboard (from CP6 v2 trim):** patrons currently land on `/` and see the staff-oriented placeholder layout. Deferred design gives them a role-differentiated view with My Active Loans and similar personal data cards. Tied to the dashboard redesign above.
- **Patron address field (from CP6 v2 trim):** `patrons.address TEXT` nullable column to support printed overdue notices. Three design options (always on / skip mail flow / optional-with-graceful-fallback) documented in `docs/cp6-planning.md`. Only relevant when the notice print system is built.
