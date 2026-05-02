# Plan: LibreShelf -- Library Management System

## Overview

LibreShelf is a self-hostable library management system built for CS408 Spring 2026.
It replaces the earlier Hello World / todo-app demo and uses the same proven tech stack
and deployment infrastructure.

LibreShelf lets a small library (school, office, personal collection) manage books,
patrons, and loans through a simple web UI. A public kiosk lets anyone browse the
catalog without logging in; patrons may optionally log in to save searches, favorite
books, and request holds on checked-out titles. All checkout and return transactions
are handled exclusively by staff on the book detail page.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | **Go 1.24** |
| Web framework | **Gin** (`github.com/gin-gonic/gin`) |
| Templating | **Go `html/template`** with layout pattern |
| Database | **SQLite** via `modernc.org/sqlite` (pure Go, no CGo) |
| CSS | **Bootstrap 5.3** (served locally -- no CDN dependency) |
| Deployment | **EC2 + systemd + nginx** |
| CI | **GitHub Actions** |

---

## Routes

| Route | Page | Access |
|-------|------|--------|
| `GET /login` | Login | Public |
| `POST /login` | Login action | Public |
| `POST /logout` | Logout action | Any logged-in user |
| `GET /` | Dashboard | Any logged-in user |
| `GET /catalog` | Catalog | Any logged-in user |
| `GET /books/:id` | Book Detail | Any logged-in user |
| `GET /kiosk` | Public kiosk catalog (anonymous; CP6) | **Public** |
| `GET /kiosk/books/:id` | Public read-only book detail (CP6) | **Public** |
| `GET /loans` | Active/overdue loan list view (CP6) | Admin + staff |
| `POST /books/:id/checkout` | Check out a copy to a patron (CP6) | Admin + staff |
| `POST /loans/:id/return` | Return a checked-out copy (CP6) | Admin + staff |
| `GET /my/loans` | Patron's own active loans (CP6) | Patron only |
| `POST /kiosk/favorites` | Save favorite *(deferred post-submission)* | Requires login |
| `POST /kiosk/holds` | Request hold *(deferred post-submission)* | Requires login |
| `GET /patrons` | Patrons | Admin + staff |
| `POST /patrons` | Create patron | Admin + staff |
| `POST /patrons/:id/edit` | Edit patron | Admin + staff |
| `POST /patrons/:id/delete` | Delete patron | Admin + staff |
| `GET /staff` | Staff management | Admin only |
| `POST /staff` | Create staff / admin | Admin only |
| `POST /staff/:id/edit` | Edit staff / admin | Admin only |
| `POST /staff/:id/delete` | Delete staff / admin | Admin only |
| `POST /staff/:id/password` | Reset staff password | Admin only |
| `GET /books/new` | Add-book form | Admin + staff |
| `POST /books` | Create book | Admin + staff |
| `GET /books/:id/edit` | Edit-book form | Admin + staff |
| `POST /books/:id/edit` | Update book | Admin + staff |
| `POST /books/:id/delete` | Delete book | Admin only |
| `GET /api/openlibrary/isbn/:isbn` | OL lookup proxy (JSON) | Admin + staff |
| `GET /admin` | Admin panel | Admin only |
| `GET /events` | SSE stream *(deferred post-submission)* | Any logged-in user |

---

## Database Schema

```sql
CREATE TABLE books (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    isbn               TEXT UNIQUE,              -- null for books without ISBN
    title              TEXT NOT NULL,
    publisher          TEXT,
    year               INTEGER,
    description        TEXT,
    cover_filename     TEXT,                     -- filename only, file in DATA_DIR/covers/ (default data/covers/; moved in #20)
    genre              TEXT,
    quantity_total     INTEGER DEFAULT 1,
    quantity_available INTEGER DEFAULT 1         -- decremented on checkout, incremented on return (CP6)
);

CREATE TABLE authors (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE     -- case-insensitive uniqueness per project identifier standard (#20)
);

CREATE TABLE book_authors (
    book_id   INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
    position  INTEGER NOT NULL DEFAULT 0,        -- 1-indexed author order shown on the book jacket (#20)
    PRIMARY KEY (book_id, author_id)
);

CREATE TABLE patrons (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    email       TEXT,
    phone       TEXT,
    joined_date DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata    TEXT                             -- JSON TEXT, nullable (DEC-016); UI deferred post-submission
);

CREATE TABLE loans (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id        INTEGER REFERENCES books(id),
    patron_id      INTEGER REFERENCES patrons(id),
    checked_out_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    due_date       DATETIME,
    returned_at    DATETIME                      -- NULL while still checked out
);

-- Overdue status is never stored -- always computed at query time:
-- returned_at IS NULL AND due_date < CURRENT_TIMESTAMP

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE,          -- COLLATE NOCASE fix queued for CP7; today the CreatePatron check guards case-insensitively
    password_hash TEXT NOT NULL,                 -- bcrypt hash
    role          TEXT NOT NULL CHECK(role IN ('admin', 'staff', 'patron')),
    patron_id     INTEGER REFERENCES patrons(id), -- NULL for admin and staff
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,                 -- crypto/rand generated
    user_id    INTEGER NOT NULL REFERENCES users(id),
    csrf_token TEXT NOT NULL,                    -- crypto/rand generated, bound to session (CP4, DEC-017)
    expires_at DATETIME NOT NULL                 -- canonical UTC "YYYY-MM-DD HH:MM:SS" (DEC-018)
);
```

### Seed accounts

Created automatically on first startup if they don't exist:

| Username | Password | Role | Notes |
|----------|----------|------|-------|
| `admin` | `Admin123!` | admin | Overridable via `ADMIN_PASSWORD` env var; must pass `ValidatePassword` (DEC-021) |
| `staff1` | `Staff123!` | staff | Added in CP4 with the three-role model |
| `patron1` | `Patron123!` | patron | `users.patron_id` is NULL. Seed user can log in but has no `patrons` row; tracked as a legacy cleanup item. |

---

## Directory Structure

Actual tree as of end of CP7 on `main`.

```
go-full-stack/
├── main.go                       # Entry point: router, template loading, middleware, server
├── main_test.go                  # HTTP boundary + role tests; setupTestRouter + loginAs helpers
├── db.go                         # DatabaseManager: schema creation + all CRUD methods
├── db_test.go                    # DB method tests (users, seed, transactions)
├── handlers.go                   # renderTemplate / renderPage helpers + dashboard / admin / kiosk / not-found
├── handlers_auth.go              # Login, logout, session middleware, RequireAuth / RequireStaff / RequireAdmin, CSRFProtect
├── handlers_books.go             # Catalog, detail, Create / Edit / Update / Delete, Open Library proxy
├── handlers_books_test.go        # Book handler tests (28 tests)
├── handlers_patrons.go           # Patron list, create, edit, delete
├── handlers_patrons_test.go      # Patron handler tests (13 tests)
├── handlers_staff.go             # Staff list, create, edit, delete, reset-password
├── handlers_staff_test.go        # Staff handler tests
├── handlers_loans.go             # Checkout / return (wired to book-detail scaffold) + /loans list view + /my/loans (CP6)
├── handlers_loans_test.go        # Loan handler tests + /my/loans privacy boundary tests
├── handlers_kiosk_test.go        # Kiosk public-access tests + auth-gated /books/:id regression guard
├── db_loans_test.go              # Loan DB method tests (CheckoutBook, ReturnBook, filters, counts)
├── handlers_admin.go             # Admin tools index + backup export / import (DEC-027, DEC-029)
├── handlers_admin_test.go        # Backup export / import tests incl. Zip Slip rejection
├── handlers_security_test.go     # SecurityHeaders + HSTS gating tests (DEC-028)
├── handlers_auth_test.go         # Auth middleware redirect-on-no-user + login success path
├── openlibrary_test.go           # OL fetch via httptest.Server (success / 404 / 500 / bad JSON)
├── covers_test.go                # Cover URL download via httptest.Server (success / 404 / oversize / bad-mime)
├── validators_isbn_test.go       # IsValidISBN boundaries incl. ISBN-10 X check digit
├── internal/safezip/             # ZIP extraction with Zip Slip / symlink / size protections (DEC-027)
│   ├── doc.go
│   ├── extract.go
│   └── extract_test.go           # 13 cases at 87.5% coverage including hand-rolled zip-bomb fixture
├── openlibrary.go                # Open Library API client (DEC-008)
├── covers.go                     # Cover upload + OL-URL download with MIME / size / extension validation
├── flash.go                      # HttpOnly flash cookies (success + error + detail companion)
├── validators.go                 # IsValidISBN, ValidateUsername, ValidatePassword, generateBaseUsername, normalizeFreeText
├── validators_test.go            # Validator unit tests
├── templates/
│   ├── layout.html               # Base layout with responsive offcanvas sidebar
│   ├── login.html                # Standalone login (no sidebar)
│   ├── index.html                # Dashboard
│   ├── catalog.html              # 4-wide book grid, search / filter, admin "+ Add Book" button
│   ├── book_detail.html          # Single book view + Edit / Delete affordances (type-to-confirm delete)
│   ├── book_form.html            # Shared new / edit form: ISBN + OL Lookup, cover preview, Variant B submit
│   ├── patrons.html              # Patron list + Add / Edit / Delete modals
│   ├── staff.html                # Staff list + Add / Edit / Delete / Reset Password modals
│   ├── admin.html                # Admin tools index page with cards drilling into individual tools (DEC-029)
│   ├── backup_admin.html         # Backup admin page: stats panel + export button + restore-from-backup modal (DEC-027)
│   ├── kiosk.html                # Public kiosk catalog grid (CP6, no login gate)
│   ├── kiosk_layout.html         # Public kiosk shell: no sidebar, no nav, "Public Terminal" header (CP6)
│   ├── kiosk_book_detail.html    # Public read-only book detail (CP6)
│   ├── loans.html                # Active/overdue loan list view with filter (CP6, staff/admin)
│   ├── my_loans.html             # Patron's own active loans (CP6, patron-only via RequirePatron)
│   └── error.html                # 404 / 500 error page
├── static/
│   ├── stylesheets/
│   │   ├── bootstrap.min.css     # Bootstrap 5.3 (local, offline-ready)
│   │   └── style.css             # Custom styles: sidebar, availability badges, letterboxed covers
│   ├── javascripts/
│   │   ├── bootstrap.bundle.min.js
│   │   ├── app.js                # Catalog filter, staff / patron / book modal init, OL Lookup, cover preview, live validation
│   │   └── admin_backup.js       # Backup-restore modal interlock (extracted from inline script for CSP)
│   └── images/
│       └── favicon.svg
├── data/                         # gitignored; holds database.sqlite, covers/, WAL files
├── scripts/
│   ├── install.sh                # EC2 install script
│   └── configure.sh              # EC2 configure script
├── deploy/
│   └── go-full-stack.service     # systemd unit
├── docs/                         # Plan, security, deployment, tutorials (see docs/README.md)
├── .tool-versions                # Go toolchain pin (asdf / mise)
├── CLAUDE.md                     # Session working agreements + current task list
├── DECISIONS.md                  # Numbered architectural decisions (DEC-001 through DEC-023)
├── README.md                     # Public-facing overview
├── go.mod
├── go.sum
└── .gitignore
```

---

## Checkpoint Plan

### CP1 -- Project Skeleton: Routes, Nav, Schema ✅
**Goal:** Working skeleton with all 6 routes returning placeholder pages; DB schema created on startup.

- ✅ All 6 routes added to `main.go`
- ✅ `templates/layout.html` -- nav bar with links to all 6 pages; Bootstrap served locally
- ✅ Placeholder templates created for all pages including `error.html`
- ✅ LibreShelf 5-table schema implemented in `db.go`
- ✅ Stub handlers in `handlers.go` with `DatabaseMiddleware` and `renderTemplate`
- ✅ `main_test.go` -- 3 real tests using `setupTestRouter` helper and temp database

**Verification:**
- ✅ `go build -o go-full-stack .` compiles cleanly
- ✅ All 6 routes return 200 with the nav bar visible
- ✅ `data/database.sqlite` created with correct 5-table schema
- ✅ `go test ./...` passes -- 3 tests: `TestIndexRoute`, `TestAllRoutesReturn200`, `TestNotFoundReturns404`
- ✅ Deployed to EC2 (URL available on request)

---

### CP2 -- Authentication & Session Management ✅
**Goal:** All routes protected by login. Admin and patron roles enforced. Seed accounts created on first run.

- ✅ `layout.html`: Updated to sidebar navigation based on wireframes (replaces top navbar)
- ✅ `handlers_auth.go`: `HandleLogin` (GET/POST), `HandleLogout`
- ✅ `db.go`: `users` and `sessions` tables, `CreateUser()`, `GetUserByUsername()`, `CreateSession()`, `GetSession()`, `DeleteSession()`, `SeedDefaultUsers()`
- ✅ `db.go`: Enable WAL mode (`PRAGMA journal_mode=WAL`) -- required by spec
- ✅ `templates/login.html`: login form
- ✅ `main.go`: `RequireAuth()` and `RequireAdmin()` middleware applied to all routes
- ✅ Dependency: `golang.org/x/crypto/bcrypt` for password hashing

**Access control applied (final, end of CP7):**
| Middleware | Routes |
|-----------|--------|
| `RequireAuth` + `DBReadLock` | `/`, `/catalog`, `/books/:id`, `POST /logout` |
| `RequirePatron` + `DBReadLock` | `/my/loans` |
| `RequireStaff` (admin + staff) + `DBReadLock` | `/patrons` + patron CRUD, `/books/new`, `POST /books`, book edit, `/api/openlibrary/isbn/:isbn`, `/loans`, `POST /books/:id/checkout`, `POST /loans/:id/return` |
| `RequireAdmin` + `DBReadLock` | `/staff` + staff CRUD + staff password reset, `POST /books/:id/delete`, `/admin`, `/admin/backup`, `GET /admin/backup/export` |
| `RequireAdmin` (no `DBReadLock`) | `POST /admin/backup/import` -- takes write lock directly to swap the DB file (DEC-027) |
| Public | `GET /login`, `POST /login`, `GET /kiosk`, `GET /kiosk/books/:id`, static files |
| `RequireAuth` (kiosk, *deferred post-submission*) | `POST /kiosk/favorites`, `POST /kiosk/holds` |

**Seed accounts (created on first run):**
- `admin` / `Admin123!` (role: admin) -- password overridable via `ADMIN_PASSWORD` env var (validated on startup; see DEC-021)
- `patron1` / `Patron123!` (role: patron) -- `users.patron_id` is NULL; the seeded user can log in but has no `patrons` row. New patrons created via `HandlePatronCreate` get a linked `patrons` row per DEC-022. The seed mismatch is legacy and doesn't affect login / role gating.

**Verification:**
- ✅ `GET /` redirects to `/login` when not logged in
- ✅ `GET /kiosk` loads without login
- ✅ Admin can log in and access all routes
- ✅ Patron gets 403 on `/patrons` and `/admin`
- ✅ `POST /logout` clears session and redirects to `/login`

---

### CP3 -- Book Catalog & Detail Pages ✅
**Goal:** `/catalog` shows real books from DB; `/books/:id` shows full book detail.

- ✅ `db.go`: Updated `books` schema -- `quantity_total`, `quantity_available`, `cover_filename`, `publisher`, `description`, `genre`
- ✅ `db.go`: Updated `patrons` schema -- added `phone` and `joined_date` columns
- ✅ `db.go`: Added `UNIQUE` constraint on `authors.name` and `books.isbn`
- ✅ `db.go`: `Book`, `Author`, `LoanRecord` structs; `GetAllBooks()`, `GetBookByID()`, `GetLoanHistory()`
- ✅ `db.go`: `SeedBooks()` -- 6 books including multi-author (Good Omens)
- ✅ `handlers_books.go`: `HandleCatalog`, `HandleBookDetail` with three-way error handling
- ✅ `templates/catalog.html`: 2-column card grid with search bar, genre dropdown, availability filter
- ✅ `templates/book_detail.html`: metadata grid, availability, loan history table, admin-only checkout scaffold
- ✅ `static/javascripts/app.js`: client-side catalog filtering (works instantly while all 100 seeded books are rendered in the DOM; server-side pagination + AJAX fragment-swap filter deferred post-submission, picked up when catalog exceeds what fits in a single render)
- ✅ `static/stylesheets/style.css`: availability badges, sticky sidebar
- ✅ `templates/layout.html`: role-aware sidebar labels, removed kiosk nav link
- ✅ `main.go`: template FuncMap with `deref` for nullable pointer fields
- ✅ `main_test.go`: updated template loading, added `TestBookDetailNotFoundReturns404`, `TestBookDetailNonNumericReturns404`

**Bug fixes (from code review):**
- ✅ [#28](https://github.com/timLP79/cs408-go-stack/issues/28) -- `CreateSession` error now checked, logged, and returns generic error
- ✅ [#29](https://github.com/timLP79/cs408-go-stack/issues/29) -- `SeedDefaultUsers` checks all `Scan` and `CreateUser` errors
- ✅ [#30](https://github.com/timLP79/cs408-go-stack/issues/30) -- `HandleNotFound` uses `renderTemplate` for proper layout rendering

---

### CP4 -- Security Hardening + Three-Role Model ✅
**Goal:** Lock down the foundation with CSRF, error handling, and the three-role access model before adding CRUD features.

- ✅ [#38](https://github.com/timLP79/cs408-go-stack/issues/38) -- Three-role model: admin, staff, patron
  - `users.role` CHECK constraint updated to include `'staff'`
  - `RequireStaff` middleware (allows admin + staff)
  - `RequireAdmin` refactored to chain after `RequireAuth`
  - Route groups restructured: public, auth, staff, admin
  - Sidebar nav adapts for three roles
  - Seed staff account: `staff1` / `Staff123!`
- ✅ [#31](https://github.com/timLP79/cs408-go-stack/issues/31) -- `ExecuteTemplate` errors never checked in render helpers
  - `renderTemplate` and `renderPage` now buffer template output before writing to the response
  - Failed template execution returns a clean 500 instead of a half-written page
  - Content-Type set explicitly (`text/html; charset=utf-8`) rather than relying on body sniffing
- ✅ [#32](https://github.com/timLP79/cs408-go-stack/issues/32) -- CSRF protection on all state-changing forms
  - Session-bound synchronizer token stored on the `sessions` row (see DEC-017)
  - Pre-session double-submit cookie for `POST /login` (chicken-and-egg case)
  - `CSRFProtect` middleware attached to auth/staff/admin route groups
  - `LoginCSRFProtect` middleware on `POST /login`
  - `POST /logout` moved into the auth group so it is CSRF-protected
  - `SameSite=Strict` now actually wired on the session cookie (previously documented but missing; see DEC-004 addendum)
  - Latent datetime bug in `CreateSession` fixed during integration testing (see DEC-018)
- ✅ [#33](https://github.com/timLP79/cs408-go-stack/issues/33) -- Username enumeration via login timing side-channel
  - `HandleLoginPost` always runs `bcrypt.CompareHashAndPassword` against a precomputed dummy hash when the user does not exist
  - Both branches take the same wall-clock time (verified by `TestLoginTimingIsConstant`)
- ✅ [#34](https://github.com/timLP79/cs408-go-stack/issues/34) -- Missing `lang="en"` on HTML tags (WCAG 2.1)

**Permission matrix:**

| Capability | Admin | Staff | Patron |
|---|---|---|---|
| Dashboard, catalog, book detail | Yes | Yes | Yes |
| Loan history on book detail page | Yes | Yes | No |
| Checkout / return books *(CP6)* | Yes | Yes | No |
| Book add / edit | Yes | Yes | No |
| Book delete | Yes | No | No |
| Open Library ISBN lookup | Yes | Yes | No |
| View patron list | Yes | Yes | No |
| Patron CRUD (name / email / phone) | Yes | Yes | No |
| Staff management (list / add / edit / delete / reset password) | Yes | No | No |
| Admin tools index `/admin` | Yes | No | No |
| ZIP export / import (`/admin/backup`) | Yes | No | No |

*CSV patron import and patron reset-password are deferred post-submission; patron holds and SSE availability are deferred post-submission; see "Deferred to post-submission backlog" in the rescope section below.*

All checkout and return transactions are staff-only. Patrons cannot self-checkout; their login exists for browsing, favorites, and requesting holds on the kiosk.

---

### Deadline Rescope (2026-04-18)

Project due 2026-05-01. Remaining scope was rebalanced within CP5/CP6/CP7 to fit the calendar. CP boundaries preserved; a few items moved between checkpoints and a few sub-features deferred to a post-submission backlog. See also the "Open Issues / Current Focus" section of `CLAUDE.md` for the authoritative per-session task list.

**Target close dates:** CP5 by 4/24, CP6 by 4/27, CP7 by 4/30. Buffer day 5/1.

**Deferred to post-submission backlog (not abandoned, captured for later):**

- CSV patron import (from #21). Manual entry sufficient for demo.
- Patron reset-password handler + modal (from #21 rescope). Today's recovery path is admin delete + recreate; acceptable for a small library but worth adding for UX.
- Patron metadata column UI (from #21 rescope). `patrons.metadata` stays in schema and null on create.
- Patron activate / deactivate flag (raised during #21 smoke test). Soft-deactivation for suspending patrons without destroying history; needs schema column, login-middleware rejection, session wipe on deactivate, UI toggle.
- Orphan cover cleanup on post-cover-save validation failure (from #20). Low-frequency leak when duplicate-ISBN check fires after a cover has already been saved to disk.
- Offline detection for the Open Library Lookup button (from #20). `navigator.onLine` + `online`/`offline` events plus first-failure-hides-button.
- SSE live availability updates (from #22). Page reload shows the same info.
- Patron holds on checked-out books (from #22). Favorites are enough to demonstrate optional patron login.
- Staff table responsive polish (from #39). Column overflow on narrow viewports.
- Password-reset Variant 2, server-generated temp + force-change-on-next-login (from #39). The stronger long-term posture; Variant 1 is acceptable for a small trusted staff.
- [#37](https://github.com/timLP79/cs408-go-stack/issues/37) Server-side catalog pagination and filtering, Path 2 approach (AJAX fragment swap). Current all-in-DOM render at 100 seeded books is fast and the client-side live filter feels right, so the foundation need is not urgent yet. Path 2 design: extract the catalog grid + pagination controls into a named `{{define "catalog_results"}}` template block; `HandleCatalog` branches on `X-Requested-With: XMLHttpRequest` (or a `?partial=1` query param) to render just that block for filter/pagination updates; client-side JS in `app.js` debounces the search input (~300ms), fires `fetch()` with the AJAX header, swaps `#catalog-results` innerHTML, updates the URL via `history.replaceState` (keystrokes) or `pushState` (genre/available/pagination changes); `AbortController` on each request to prevent a slow stale response clobbering a newer one; full-page form submission as a no-JS fallback. The fragment-swap infra reuses for `/loans` filtering once volume grows there too. Unblocked when the catalog routinely exceeds ~500 rows or when the CP7 close frees up a session for the work.
- [#17](https://github.com/timLP79/cs408-go-stack/issues/17) GitHub Actions deploy. Already labeled backlog.

Full details for each item live in `CLAUDE.md` "Deferred post-submission backlog"; this list is the planning-doc mirror.

---

### CP5 -- CRUD Features (Books, Patrons, Staff) ✅
**Goal:** Full CRUD for books (with Open Library API), patrons, and staff accounts. Test harness fix (#35) pulled into CP5 since handler tests for #39/#20/#21 depend on it. Closed 2026-04-18, 6 days ahead of the 4/24 target.

- ✅ [#35](https://github.com/timLP79/cs408-go-stack/issues/35) -- Test router mirror fix. `setupTestRouter` now mirrors `main.go` route groups byte-for-byte. `loginAs` and `logoutHelper` test helpers added. Three regression-pin tests cover the middleware chain.
- ✅ [#39](https://github.com/timLP79/cs408-go-stack/issues/39) -- Staff management: list, add, edit, delete, reset-password.
  - `handlers_staff.go` + `handlers_staff_test.go` -- all five handlers with one test per guard rule (self-delete, self-demote, last-admin delete/demote, role whitelist, admin-only access, IDOR on patron target)
  - Flash-cookie PRG messaging (`flash.go`), Bootstrap live validation across Add / Edit / Reset modals
  - `UpdateUserPassword` is transactional (DEC-022) and wipes target sessions on every reset
  - Safety guards: cannot delete self, cannot demote self, cannot delete/demote last admin
- ✅ [#20](https://github.com/timLP79/cs408-go-stack/issues/20) -- Book CRUD and Open Library API lookup.
  - `handlers_books.go`: `HandleBookNew`, `HandleBookCreate`, `HandleBookEdit`, `HandleBookUpdate`, `HandleBookDelete`, plus `HandleOpenLibraryLookup` (routes: `GET /api/openlibrary/isbn/:isbn`, staff+admin; `POST /books/:id/delete` admin-only; create/edit staff+admin)
  - `openlibrary.go` server-side OL client (DEC-008); `covers.go` upload + OL-URL download with size / extension / magic-byte MIME validation, storage under `DATA_DIR/covers/`; `flash.go` detail-cookie companion
  - `db.go`: `CreateBook`, `UpdateBook`, `DeleteBook`, `GetBookByISBN`, `findOrCreateAuthor` -- transactional per DEC-022 (books + authors + book_authors)
  - Shared `templates/book_form.html` renders both new and edit with Variant B two-button submit (DEC-023)
  - Cover routing on update: upload > OL URL > preserve existing, with old-file cleanup after `UpdateBook` succeeds
  - `ErrBookHasLoans` sentinel blocks delete when loans exist (surfaces as a form banner, not a 500)
  - Duplicate-ISBN guard excludes the book being edited from the conflict check
- ✅ [#21](https://github.com/timLP79/cs408-go-stack/issues/21) -- Patron management: CRUD. *(CSV import, reset-password, metadata UI, and activate/deactivate deferred post-submission.)*
  - `handlers_patrons.go`: list, add, edit, delete -- all four in the staff+admin route group
  - `db.go`: `Patron` struct (including nullable `metadata TEXT` JSON column from DEC-016), `CreatePatron` transactional (patrons + linked users row per DEC-022) with username auto-generation + collision-retry loop inside the transaction, `DeletePatron` transactional (sessions + users + patrons) with `ErrPatronHasLoans` guard mirroring the book pattern
  - `templates/patrons.html` modeled on `staff.html`: Add / Edit / Delete modals (no Reset modal per rescope), type-to-confirm delete
  - `validators.go`: `generateBaseUsername` produces first-initial + last-word lowercased with non-alphanumerics stripped; empty input rejected at handler level before DB call
  - Admin-typed temp password at create (Variant 1, DEC-021). Username is not editable post-create (rename via delete-recreate)
  - Catalog polish pass also landed alongside #21: 4-wide grid on desktop, letterboxed covers against near-black slot (`#212529`) with natural aspect preserved

**Open Library API:**
```
GET https://openlibrary.org/api/books?bibkeys=ISBN:<isbn>&format=json&jscmd=data
```
Returns: title, authors, cover URL, publish year. Called server-side; result forwarded as JSON to client JS for form pre-fill.

---

### CP6 -- Loans + Kiosk + Dashboard ✅
**Closed 2026-04-25 via PR #42, 169 tests passing.** Scope disciplined via the v2 reality-check on 2026-04-19 and refined on 2026-04-20 -- workflow polish (rapid-scan portal, sidebar restructure, fuller dashboard redesign with mini-lists, printed overdue notices) remains deferred post-submission. Server-side catalog pagination also deferred post-submission as Path 2 (AJAX fragment swap). Full plan and deferred-design notes in [`cp6-planning.md`](./cp6-planning.md). The original-scope notes below describe what landed.

- [#22](https://github.com/timLP79/cs408-go-stack/issues/22) -- Loan system (trimmed)
  - `db.go`: loan schema (`due_date DATE NOT NULL`, `returned_at DATETIME`, `fine_cents INTEGER NOT NULL DEFAULT 0` future hook), plus DB methods `CheckoutBook`, `ReturnBook`, `GetActiveLoans`, `GetOverdueLoans`, `GetLoanHistoryByBook`, `GetLoanHistoryByPatron`. Both writes transactional (loan row + `quantity_available` adjustment in one tx, per DEC-022).
  - No `status` column -- the three states (active, returned, overdue) are expressible from `returned_at` and `due_date`. Overdue derived: `returned_at IS NULL AND due_date < DATE('now')`. See DEC-024.
  - `handlers_loans.go`: `HandleCheckout` and `HandleReturn` wired to the existing (currently disabled) scaffold on the book detail page. Staff-only route group. Sentinel errors: `ErrNoCopiesAvailable`, `ErrLoanAlreadyReturned`, `ErrPatronNotFound`. Flash-cookie messaging on success/failure.
  - `templates/loans.html` + route `/loans?filter=active|overdue` -- single-template list view, sortable by due date, each row has a Return button. See DEC-025.
  - `handlers_books.go`: `HandleKiosk` -- public anonymous catalog browse (no auth required), reuses catalog grid minus staff controls. No patron login gate in CP6; no favorites by default (favorites only if time permits after items 1-5 ship).
  - Patrons cannot self-checkout; admin/staff select a patron and transact via the book-detail form.
  - Rapid-scan `/checkout` and `/checkin` portal deferred post-submission (earns its place at volume, not a CP6 foundation need).
- Dashboard: role-differentiated essential card set (sequenced after the loan schema lands, since most cards depend on loan data)
  - **Staff / admin view (three cards):** Overdue (count with danger styling when `> 0`; links to `/loans?filter=overdue`); Active Loans (count; links to `/loans?filter=active`); Out of Stock (count of titles where `quantity_available = 0`; links to filtered catalog).
  - **Patron view (one card):** My Active Loans (count of the patron's non-returned loans + the next due date as secondary text; links to a filtered `/loans` scoped to the patron). No mini-list rendering in CP6 -- single-count-plus-next-date card shape matches the existing big-number visual language.
  - Role gating via `{{if eq .User.Role "patron"}}...{{end}}` blocks in `templates/index.html`. No CSS restructuring; reuse current card component.
  - Explicitly cut for CP6: Books count, Patrons count, Staff count (low-signal decoration per the deferred dashboard-redesign analysis); Today's Activity (needs loan activity log query, not essential); Favorites card (feature itself is "if time permits"); My Holds placeholder (holds feature is deferred, so no card).

**DECs to write in session 2 (design):** DEC-024 (loan state model via columns, no status column), DEC-025 (single `/loans` page with filter, no per-patron grouping or print CSS), DEC-026 (`fine_cents` reservation, no fine feature in CP6).

---

### CP7 -- Admin Panel + Security Hardening + Deploy ✅
**Closed 2026-05-01 across PRs #74 / #75 / #76, deployed to EC2 same day.**

- ✅ [#23](https://github.com/timLP79/cs408-go-stack/issues/23) -- Admin panel: ZIP export and import (PR #74, DEC-027)
  - `handlers_admin.go`: `GET /admin/backup` (stats panel), `GET /admin/backup/export` (VACUUM INTO snapshot + ZIP), `POST /admin/backup/import` (validated extract + atomic file swap with `.bak` rollback)
  - `internal/safezip/`: new package handling Zip Slip / symlink / absolute-path / size limits with two-pass validation. 13 tests at 87.5% coverage.
  - `templates/backup_admin.html`: stats card, download button, restore-from-backup modal with checkbox interlock
  - Live sessions preserved across the swap so the admin clicking restore stays logged in (`DumpSessions` -> swap -> `RestoreSessions` filtered to live users)
- ✅ Admin tools-index pattern (PR #74 follow-up redesign, DEC-029)
  - `/admin` is now a tools-index page with cards drilling into dedicated `/admin/<tool>` routes; lightweight settings will live inline as future toggles land
  - `/admin` moved from staff-accessible to admin-only
- ✅ [#24](https://github.com/timLP79/cs408-go-stack/issues/24) -- Testing, polish, and deploy (PR #75 + follow-up `af31e3d`, DEC-028)
  - `SecurityHeaders` middleware applied router-wide: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: same-origin`, locked-down CSP, `Strict-Transport-Security` gated on `APP_ENV=production`
  - `router.SetTrustedProxies([]string{"127.0.0.1"})` so Gin only honors `X-Forwarded-For` from the local nginx proxy
  - Go 1.25.0 -> 1.25.9 toolchain bump cleared 19 stdlib CVEs flagged by `govulncheck`. Pinned in `.tool-versions` and `go.mod`.
  - `go mod verify` + `govulncheck ./...` documented as pre-deploy gates in `docs/deployment.md`
  - nginx `client_max_body_size 100M` added to deployment guide after a live 413 on backup import (`af31e3d`)
  - Final EC2 redeploy verified end-to-end (security headers visible in DevTools, no CSP violations, backup admin works on prod)
- ✅ [#62](https://github.com/timLP79/cs408-go-stack/issues/62) (cs408-go-stack-al3) -- Test coverage push (PR #76, partial)
  - 61.2% baseline -> 67.6% (+6.4%); `internal/safezip` held at 87.5%
  - **75% headline target NOT reached.** Reaching it would require fault-injection scaffolding for handler error paths (`HandleBookEdit/Update`, `HandleStaffDelete`, `HandleBackupImport` rollback) which did not fit the CP7 deadline.
  - Mandatory acceptance items shipped: Zip Slip rejection test for admin import (`TestBackupImport_RejectsZipSlip`), `httptest.NewServer` for OL paths (`TestFetchOpenLibraryBook_*`), `httptest.NewServer` for cover-URL paths (`TestSaveCoverFromURL_*`)
- ~~[#36](https://github.com/timLP79/cs408-go-stack/issues/36) -- Empty `app.js` loaded on every page~~ (resolved in CP3)

---

## Security Plan

Security is addressed incrementally as each feature is built -- not as an afterthought in CP7.
For full details see [`docs/security.md`](./security.md).

### Already protected by design
- **XSS** -- Go's `html/template` auto-escapes all output by default. Unlike string concatenation, templates cannot inject raw HTML unless you explicitly use `template.HTML`. No extra work needed.
- **CDN supply chain** -- Bootstrap is served locally, not from a CDN. No third-party script can be injected by compromising an external server.
- **Foreign keys** -- SQLite foreign key enforcement is enabled on startup, preventing orphaned records.

### Per-checkpoint security work

| CP | Risk | Mitigation |
|----|------|-----------|
| CP2 ✅ | Weak password storage | Use `bcrypt` -- never store plain text or MD5/SHA passwords |
| CP2 ✅ | Session hijacking | `HttpOnly`, `Secure`, `SameSite=Strict` cookie; server-side session store (SameSite=Strict actually wired in CP4, see DEC-004 addendum) |
| CP2 ✅ | Session fixation | Regenerate session token after successful login |
| CP2 | Brute force login | Bcrypt's cost factor adds natural delay; rate limit `/login` POST still CP7 |
| CP3 ✅ | SQL injection via book/patron IDs | Always use parameterized queries (`?` placeholders) -- never string concatenation |
| CP4 ✅ | CSRF on state-changing forms | Session-bound synchronizer token for authenticated routes, pre-session double-submit cookie for login (#32, DEC-017) |
| CP4 ✅ | Username enumeration via login timing | `HandleLoginPost` always runs bcrypt against a dummy hash when the user does not exist (#33) |
| CP5 ✅ | File upload (cover images) | `covers.go` validates size (2 MB cap), extension whitelist (jpg/png/webp), and magic-byte MIME via `http.DetectContentType`; sanitized filename is a random 32-hex generated server-side (#20) |
| CP5 ✅ | Open Library proxy | `IsValidISBN` runs server-side before any outbound request; `HandleOpenLibraryLookup` rejects malformed ISBNs with 400 before opening a socket (#20) |
| CP5 ✅ | PII exposure (patron emails) | `email` column stays optional; handlers never log patron data; the flash detail cookie is URL-escaped but still avoids serializing email / phone (#21) |
| CP6 | Loan IDOR on return | Return handler verifies the submitted `loan_id` belongs to an active (non-returned) loan; no caller-controlled patron_id on the return path |
| Deferred | Hold request abuse | Validate patron login before allowing holds; rate limit hold requests |
| Deferred | SSE data exposure | Event stream must not include patron PII -- book ID and availability only |
| CP7 | Zip Slip (path traversal) | Validate every file path in uploaded ZIP before extracting; reject `../` paths |
| CP7 | Malicious ZIP import | Validate DB schema after import before bringing app back online |
| CP7 | HTTP security headers | Add middleware for `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` |
| CP7 | Gin proxy warning | Configure `router.SetTrustedProxies([]string{"127.0.0.1"})` for EC2/nginx setup |
| CP7 | HTTPS | Optional -- requires a domain name; Let's Encrypt does not issue certs for bare IP addresses. HTTP-only is acceptable for this class deployment. |
| CP7 | Dependency audit | Run `go mod verify` and check for known CVEs before final deploy |

### Session hijacking -- design (implemented in CP2)
LibreShelf uses server-side sessions with secure cookies.

- Session tokens generated with `crypto/rand` (cryptographically secure)
- Cookie attributes: `HttpOnly`, `SameSite=Strict`; `Secure` flag is environment-aware (disabled for HTTP-only deployments)
- Session token regenerated after login (prevents session fixation)
- Sessions stored in the `sessions` DB table -- can be invalidated server-side
- Short expiry (8 hours) with no sliding renewal -- re-login required
- CSRF tokens on all state-changing forms (`POST`, `PUT`, `DELETE`)

### Sensitive data at rest
- `data/database.sqlite` is gitignored -- never committed to the repo
- Patron emails are optional and never logged
- The `data/` directory should have restricted permissions on the server (`chmod 700 data/`)

---

## Key Design Decisions

### 1. Bootstrap served locally (diverges from spec)
The project specification listed "Bootstrap 5 via CDN" but Bootstrap is served locally from
`static/stylesheets/` and `static/javascripts/`. This is more consistent with the offline-first
architecture and eliminates CDN supply chain risk. The spec PDF cannot be changed but this
decision is intentional and documented here.

---

### 3. Flat package structure
All `.go` files in `package main`, split by concern using filename suffix
(`handlers_books.go`, `handlers_patrons.go`, etc.) rather than sub-packages.
The app is medium-sized -- sub-packages would add indirection without benefit.

### 4. SSE for real-time availability
On staff checkout/return (book detail page), the server pushes a Server-Sent Events
message to all connected browsers. SSE is one-way (server → browser) and fits the
use case exactly. No WebSocket needed.

### 5. Open Library API (server-side proxy)
ISBN metadata is fetched server-side to avoid CORS issues and to keep the client
JS simple. The handler proxies the request and returns clean JSON to the form.

### 6. ZIP export using standard library
`archive/zip` (standard library) is sufficient. No third-party dependency needed
for backup/restore.

### 7. Template loading
Same pattern as the current codebase: `map[string]*template.Template`, one entry
per page, each paired with `layout.html`. The `renderTemplate()` helper executes
the `"layout"` template, which pulls in the page's `"content"` block.

### 8. SQLite driver
`modernc.org/sqlite` registers as driver `"sqlite"` (not `"sqlite3"`). Pure Go --
no CGo, no system library dependency.

---

## Verification (end of CP1)

```bash
go build -o go-full-stack .         # must compile cleanly
go run .                             # server starts on port 3000
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/          # 200
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/catalog   # 200
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/patrons   # 200
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/admin     # 200
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/kiosk     # 200
sqlite3 data/database.sqlite ".schema"   # shows 5 tables
go test ./...                            # passes
```
