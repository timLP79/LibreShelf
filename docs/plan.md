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
| `GET /kiosk` | Kiosk | **Public** (login optional) |
| `POST /kiosk/favorites` | Save favorite | Login optional (silently no-op if not logged in) |
| `POST /kiosk/holds` | Request hold | Requires login |
| `GET /patrons` | Patrons | Admin only |
| `GET /admin` | Admin | Admin only |
| `GET /events` | SSE stream | Any logged-in user |

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
    cover_filename     TEXT,                     -- filename only, file in static/covers/
    genre              TEXT,
    quantity_total     INTEGER DEFAULT 1,
    quantity_available INTEGER DEFAULT 1         -- decremented on checkout, incremented on return
);

CREATE TABLE authors (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE book_authors (
    book_id   INTEGER REFERENCES books(id),
    author_id INTEGER REFERENCES authors(id),
    PRIMARY KEY (book_id, author_id)
);

CREATE TABLE patrons (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    email       TEXT,
    phone       TEXT,
    joined_date TEXT    -- ISO 8601 date string
);

CREATE TABLE loans (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id        INTEGER REFERENCES books(id),
    patron_id      INTEGER REFERENCES patrons(id),
    checked_out_at TEXT,   -- ISO 8601 timestamp
    due_date       TEXT,   -- ISO 8601 date string
    returned_at    TEXT    -- NULL if still checked out
);

-- Overdue status is never stored -- always computed at query time:
-- returned_at IS NULL AND due_date < CURRENT_TIMESTAMP

CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,         -- bcrypt hash
    role TEXT NOT NULL CHECK(role IN ('admin', 'staff', 'patron')),
    patron_id INTEGER REFERENCES patrons(id),  -- NULL for admin and staff
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,              -- crypto/rand generated
    user_id INTEGER NOT NULL REFERENCES users(id),
    csrf_token TEXT NOT NULL,            -- crypto/rand generated, bound to session (CP4, DEC-017)
    expires_at DATETIME NOT NULL         -- canonical UTC "YYYY-MM-DD HH:MM:SS" (DEC-018)
);
```

### Seed accounts

Created automatically on first startup if they don't exist:

| Username | Password | Role | Notes |
|----------|----------|------|-------|
| `admin` | `admin123` | admin | Overridable via `ADMIN_PASSWORD` env var |
| `staff1` | `staff123` | staff | Added in CP4 with the three-role model |
| `patron1` | `patron123` | patron | Not linked to a patron record yet (CP5) |

---

## Directory Structure

```
go-full-stack/
├── main.go                    # Entry point: router, template loading, middleware, server
├── db.go                      # DatabaseManager: schema creation + all CRUD methods
├── handlers.go                # HTTP handler functions for all pages
├── handlers_auth.go           # Login, logout, session management
├── handlers_books.go          # Book-specific handlers (catalog, detail, CRUD)
├── handlers_patrons.go        # Patron handlers
├── handlers_loans.go          # Loan/kiosk handlers + SSE endpoint
├── handlers_admin.go          # Admin handlers (ZIP export, import, Open Library proxy)
├── templates/
│   ├── layout.html            # Base layout with sidebar nav (wireframe-based)
│   ├── login.html             # Login page
│   ├── index.html             # Dashboard page
│   ├── catalog.html           # Book catalog list
│   ├── book_detail.html       # Single book view
│   ├── patrons.html           # Patron list
│   ├── admin.html             # Admin panel
│   ├── kiosk.html             # Public catalog browse (optional patron login)
│   └── error.html             # 404/500 error page
├── static/
│   ├── stylesheets/
│   │   └── style.css          # Custom styles (minimal, Bootstrap handles most)
│   ├── javascripts/
│   │   └── app.js             # Client JS (catalog filtering; SSE listener added in CP6)
│   ├── images/
│   └── favicon.svg
├── scripts/
│   ├── install.sh             # EC2 install script
│   └── configure.sh           # EC2 configure script
├── deploy/
│   └── go-full-stack.service  # systemd unit
├── docs/
│   ├── plan.md                # This file
│   └── week6/deployment.md    # EC2 + nginx + systemd deployment guide
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

### CP2 -- Authentication & Session Management
**Goal:** All routes protected by login. Admin and patron roles enforced. Seed accounts created on first run.

- `layout.html`: Updated to sidebar navigation based on wireframes (replaces top navbar)
- `handlers_auth.go`: `HandleLogin` (GET/POST), `HandleLogout`
- `db.go`: `users` and `sessions` tables, `CreateUser()`, `GetUserByUsername()`, `CreateSession()`, `GetSession()`, `DeleteSession()`, `SeedDefaultUsers()`
- `db.go`: Enable WAL mode (`PRAGMA journal_mode=WAL`) -- required by spec
- `templates/login.html`: login form
- `main.go`: `RequireAuth()` and `RequireAdmin()` middleware applied to all routes
- Dependency: `golang.org/x/crypto/bcrypt` for password hashing

**Access control applied:**
| Middleware | Routes |
|-----------|--------|
| `RequireAuth` | `/`, `/catalog`, `/books/:id`, `/events` |
| `RequireAdmin` | `/patrons`, `/admin`, all CRUD endpoints |
| Public | `/login`, `/logout`, `GET /kiosk`, static files |
| `LoadUser` (optional) | `POST /kiosk/favorites` |
| `RequireAuth` (kiosk) | `POST /kiosk/holds` |

**Seed accounts (created on first run):**
- `admin` / `admin123` (role: admin) -- password overridable via `ADMIN_PASSWORD` env var
- `patron1` / `patron123` (role: patron) -- linked to a seed patron record

**Verification:**
- `GET /` redirects to `/login` when not logged in
- `GET /kiosk` loads without login
- Admin can log in and access all routes
- Patron gets 403 on `/patrons` and `/admin`
- `POST /logout` clears session and redirects to `/login`

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
- ✅ `static/javascripts/app.js`: client-side catalog filtering (temporary, replaced by server-side in CP7)
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
  - Seed staff account: `staff1` / `staff123`
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
| Checkout/return books | Yes | Yes | Yes (self) |
| Book CRUD | Yes | Yes | No |
| View patron list | Yes | Yes | No |
| Patron CRUD | Yes | No | No |
| CSV patron import | Yes | Yes | No |
| Admin panel (export/import) | Yes | Yes | No |
| Staff management | Yes | No | No |

---

### CP5 -- CRUD Features (Books, Patrons, Staff)
**Goal:** Full CRUD for books (with Open Library API), patrons (with metadata and CSV import), and staff accounts.

- [#20](https://github.com/timLP79/cs408-go-stack/issues/20) -- Book CRUD and Open Library API lookup
  - `handlers_books.go`: `POST /books`, `PUT /books/:id`, `DELETE /books/:id`
  - `handlers_admin.go`: `GET /api/openlibrary?isbn=...` (server-side proxy)
  - `db.go`: `CreateBook()`, `UpdateBook()`, `DeleteBook()`
- [#21](https://github.com/timLP79/cs408-go-stack/issues/21) -- Patron management: CRUD, metadata, and CSV import
  - `handlers_patrons.go`: list, add, edit, delete, CSV import
  - `db.go`: `Patron` struct with `metadata TEXT` (JSON, nullable), all patron CRUD methods
  - `templates/patrons.html`: patron list with modals, CSV upload
  - Creating a patron also creates a linked `users` record (patron role)
  - Auto-generated username: first-initial + last-name with collision handling
  - Default password: `changeme` (bcrypt-hashed)
- [#39](https://github.com/timLP79/cs408-go-stack/issues/39) -- Staff management: list, add, edit, delete
  - `handlers_staff.go`: list, add, edit, delete staff accounts
  - `templates/staff.html`: staff list with modals
  - Safety guards: cannot delete self, cannot delete last admin

**Open Library API:**
```
GET https://openlibrary.org/api/books?bibkeys=ISBN:<isbn>&format=json&jscmd=data
```
Returns: title, authors, cover URL, publish year. Called server-side; result forwarded as JSON to client JS for form pre-fill.

---

### CP6 -- Loans + Kiosk + SSE
**Goal:** Kiosk provides public catalog browse with optional patron login. Checkout/return available to all roles. SSE pushes live availability updates. Catalog gets server-side pagination.

- [#22](https://github.com/timLP79/cs408-go-stack/issues/22) -- Loan system: kiosk browse, holds, and SSE availability
  - `handlers_loans.go`: `GET /events` SSE endpoint
  - `handlers_books.go`: `HandleKiosk` -- public catalog browse (no auth required)
  - `db.go`: `GetActiveLoans()`, `GetLoanHistory()`
  - `templates/kiosk.html`: public browse page; optional login for favorites and holds
  - Checkout/return on book detail page: admin and staff pick a patron, patron self-checkouts
- [#37](https://github.com/timLP79/cs408-go-stack/issues/37) -- Server-side pagination and filtering for catalog (replaces CP3 client-side filtering)

**SSE protocol:**
- Endpoint: `GET /events`
- Message format: `data: book_id=N available=0\n\n`
- Catalog page JS listens and updates availability badges without page reload

---

### CP7 -- Admin Panel + Testing + Deploy
**Goal:** ZIP export/import, test coverage, security hardening, final EC2 redeploy.

- [#23](https://github.com/timLP79/cs408-go-stack/issues/23) -- Admin panel: ZIP export and import
  - `handlers_admin.go`: `GET /admin/export`, `POST /admin/import`
  - `templates/admin.html`: export button, import file picker, system stats
  - Uses Go standard library `archive/zip` -- no extra dependencies
  - ZIP contains: SQLite database file + cover images from `static/images/covers/`
- [#35](https://github.com/timLP79/cs408-go-stack/issues/35) -- Test router does not mirror production middleware (false-positive tests)
- [#24](https://github.com/timLP79/cs408-go-stack/issues/24) -- Testing, polish, and deploy
  - `db_test.go`: unit tests for all DB methods including auth
  - `handlers_test.go`: integration tests for all HTTP handlers including auth flows
  - `scripts/install.sh`, `scripts/configure.sh`: EC2 automation scripts
  - Security headers middleware, trusted proxies config
  - Run `go mod verify` and dependency audit
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
| CP2 | Weak password storage | Use `bcrypt` -- never store plain text or MD5/SHA passwords |
| CP2 | Session hijacking | `HttpOnly`, `Secure`, `SameSite=Strict` cookie; server-side session store (SameSite=Strict actually wired in CP4, see DEC-004 addendum) |
| CP2 | Session fixation | Regenerate session token after successful login |
| CP2 | Brute force login | Bcrypt's cost factor adds natural delay; rate limit `/login` POST in CP7 |
| CP3 | SQL injection via book/patron IDs | Always use parameterized queries (`?` placeholders) -- never string concatenation |
| CP4 ✅ | CSRF on state-changing forms | Session-bound synchronizer token for authenticated routes, pre-session double-submit cookie for login (#32, DEC-017) |
| CP4 ✅ | Username enumeration via login timing | `HandleLoginPost` always runs bcrypt against a dummy hash when the user does not exist (#33) |
| CP5 | File upload (cover images) | Validate MIME type, restrict extensions, limit file size, sanitize filename |
| CP5 | Open Library proxy | Validate ISBN format server-side before making outbound request |
| CP5 | PII exposure (patron emails) | Never log patron data; keep `email` field optional |
| CP6 | Hold request abuse | Validate patron login before allowing holds; rate limit hold requests |
| CP6 | SSE data exposure | Event stream must not include patron PII -- book ID and availability only |
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
