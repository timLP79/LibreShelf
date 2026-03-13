# Plan: LibreShelf — Library Management System

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
| CSS | **Bootstrap 5.3** (served locally — no CDN dependency) |
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

-- Overdue status is never stored — always computed at query time:
-- returned_at IS NULL AND due_date < CURRENT_TIMESTAMP

CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,         -- bcrypt hash
    role TEXT NOT NULL CHECK(role IN ('admin', 'patron')),
    patron_id INTEGER REFERENCES patrons(id),  -- NULL for admin users
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,              -- crypto/rand generated
    user_id INTEGER REFERENCES users(id),
    expires_at DATETIME NOT NULL
);
```

### Seed accounts

Created automatically on first startup if they don't exist:

| Username | Password | Role | Notes |
|----------|----------|------|-------|
| `admin` | `admin123` | admin | Overridable via `ADMIN_PASSWORD` env var |
| `patron1` | `patron123` | patron | Linked to a seed patron record |

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
│   │   └── app.js             # Client JS (SSE listener for availability updates)
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

### CP1 — Project Skeleton: Routes, Nav, Schema ✅
**Goal:** Working skeleton with all 6 routes returning placeholder pages; DB schema created on startup.

- ✅ All 6 routes added to `main.go`
- ✅ `templates/layout.html` — nav bar with links to all 6 pages; Bootstrap served locally
- ✅ Placeholder templates created for all pages including `error.html`
- ✅ LibreShelf 5-table schema implemented in `db.go`
- ✅ Stub handlers in `handlers.go` with `DatabaseMiddleware` and `renderTemplate`
- ✅ `main_test.go` — 3 real tests using `setupTestRouter` helper and temp database

**Verification:**
- ✅ `go build -o go-full-stack .` compiles cleanly
- ✅ All 6 routes return 200 with the nav bar visible
- ✅ `data/database.sqlite` created with correct 5-table schema
- ✅ `go test ./...` passes — 3 tests: `TestIndexRoute`, `TestAllRoutesReturn200`, `TestNotFoundReturns404`
- ✅ Deployed to EC2 (URL available on request)

---

### CP2 — Authentication & Session Management
**Goal:** All routes protected by login. Admin and patron roles enforced. Seed accounts created on first run.

- `layout.html`: Updated to sidebar navigation based on wireframes (replaces top navbar)
- `handlers_auth.go`: `HandleLogin` (GET/POST), `HandleLogout`
- `db.go`: `users` and `sessions` tables, `CreateUser()`, `GetUserByUsername()`, `CreateSession()`, `GetSession()`, `DeleteSession()`, `SeedDefaultUsers()`
- `db.go`: Enable WAL mode (`PRAGMA journal_mode=WAL`) — required by spec
- `templates/login.html`: login form
- `main.go`: `RequireAuth()` and `RequireAdmin()` middleware applied to all routes
- Dependency: `golang.org/x/crypto/bcrypt` for password hashing

**Access control applied:**
| Middleware | Routes |
|-----------|--------|
| `RequireAuth` | `/`, `/catalog`, `/books/:id`, `/events` |
| `RequireAdmin` | `/patrons`, `/admin`, all CRUD endpoints |
| Public | `/login`, `/logout`, `GET /kiosk`, static files |
<<<<<<< HEAD
| `LoadUser` (optional) | `POST /kiosk/favorites` |
| `RequireAuth` (kiosk) | `POST /kiosk/holds` |
=======
| Login optional (`LoadUser`) | `POST /kiosk/favorites` |
| `RequireAuth` (kiosk action) | `POST /kiosk/holds` |
>>>>>>> 3b00650 (Update CP2 access control table to reflect public kiosk design)

**Seed accounts (created on first run):**
- `admin` / `admin123` (role: admin) — password overridable via `ADMIN_PASSWORD` env var
- `patron1` / `patron123` (role: patron) — linked to a seed patron record

**Verification:**
- `GET /` redirects to `/login` when not logged in
- `GET /kiosk` loads without login
- Admin can log in and access all routes
- Patron gets 403 on `/patrons` and `/admin`
- `POST /logout` clears session and redirects to `/login`

---

### CP3 — Book Catalog & Detail Pages
**Goal:** `/catalog` shows real books from DB; `/books/:id` shows full book detail.

- `db.go`: Update `books` schema to match spec — `quantity_total`, `quantity_available`, `cover_filename`, `publisher`, `description`, `genre` (replaces CP1 stub columns). Delete existing `data/database.sqlite` before first run to apply changes.
- `db.go`: Update `patrons` schema — add `phone` and `joined_date` columns
- `handlers_books.go`: `HandleCatalog`, `HandleBookDetail`
- `db.go`: `GetAllBooks()`, `GetBookByID()`
- `templates/catalog.html`: searchable/filterable book list
- `templates/book_detail.html`: book info, availability, loan history

---

### CP4 — Book CRUD + Open Library API
**Goal:** Admin can add, edit, and delete books. ISBN lookup auto-fills metadata.

- `handlers_books.go`: `POST /books`, `PUT /books/:id`, `DELETE /books/:id`
- `handlers_admin.go`: `GET /api/openlibrary?isbn=...` (server-side proxy)
- `db.go`: `CreateBook()`, `UpdateBook()`, `DeleteBook()`

**Open Library API:**
```
GET https://openlibrary.org/api/books?bibkeys=ISBN:<isbn>&format=json&jscmd=data
```
Returns: title, authors, cover URL, publish year. Called server-side; result forwarded as JSON to client JS for form pre-fill.

---

### CP5 — Patron Management
**Goal:** `/patrons` shows patron list with full CRUD. Admin only.

- `handlers_patrons.go`: list, add, edit, delete
- `db.go`: `GetAllPatrons()`, `GetPatronByID()`, `CreatePatron()`, `UpdatePatron()`, `DeletePatron()`
- `templates/patrons.html`: patron list with inline forms
- Creating a patron also creates a linked `users` record (patron role)

---

### CP6 — Loans & Kiosk + SSE
**Goal:** Kiosk provides public catalog browse with optional login for patron features. SSE pushes live availability updates to all connected browsers.

- `handlers_loans.go`: `GET /events` SSE endpoint
- `handlers_books.go`: `HandleKiosk` — public catalog browse (no auth required)
- `db.go`: `GetActiveLoans()`, `GetLoanHistory()`
- `templates/kiosk.html`: public browse page; optional login to save searches, favorites, and request holds on checked-out titles
- Note: checkout and return are staff-only actions on the book detail page — not available on the kiosk

**SSE protocol:**
- Endpoint: `GET /events`
- Message format: `data: book_id=N available=0\n\n`
- Catalog page JS listens and updates availability badges without page reload

---

### CP7 — Admin Panel (ZIP Export/Import)
**Goal:** Admin can export the entire DB as a ZIP and import it back.

- `handlers_admin.go`: `GET /admin/export`, `POST /admin/import`
- `templates/admin.html`: export button, import file picker, system stats
- Uses Go standard library `archive/zip` — no extra dependencies

ZIP contains: SQLite database file + cover images from `static/images/covers/`

---

### CP8 — Testing, Polish & Deploy
**Goal:** Test coverage, UI cleanup, security hardening, final EC2 redeploy.

- `db_test.go`: unit tests for all DB methods including auth
- `handlers_test.go`: integration tests for all HTTP handlers including auth flows
- `scripts/install.sh`, `scripts/configure.sh`: EC2 automation scripts
- Security headers middleware, trusted proxies config, HTTPS setup

---

## Security Plan

Security is addressed incrementally as each feature is built — not as an afterthought in CP7.
For full details see [`docs/security.md`](./security.md).

### Already protected by design
- **XSS** — Go's `html/template` auto-escapes all output by default. Unlike string concatenation, templates cannot inject raw HTML unless you explicitly use `template.HTML`. No extra work needed.
- **CDN supply chain** — Bootstrap is served locally, not from a CDN. No third-party script can be injected by compromising an external server.
- **Foreign keys** — SQLite foreign key enforcement is enabled on startup, preventing orphaned records.

### Per-checkpoint security work

| CP | Risk | Mitigation |
|----|------|-----------|
| CP2 | Weak password storage | Use `bcrypt` — never store plain text or MD5/SHA passwords |
| CP2 | Session hijacking | `HttpOnly`, `Secure`, `SameSite=Strict` cookie; server-side session store |
| CP2 | Session fixation | Regenerate session token after successful login |
| CP2 | Brute force login | Bcrypt's cost factor adds natural delay; rate limit `/login` POST in CP8 |
| CP3 | SQL injection via book/patron IDs | Always use parameterized queries (`?` placeholders) — never string concatenation |
| CP4 | File upload (cover images) | Validate MIME type, restrict extensions, limit file size, sanitize filename |
| CP4 | Open Library proxy | Validate ISBN format server-side before making outbound request |
| CP5 | PII exposure (patron emails) | Never log patron data; keep `email` field optional |
| CP6 | Hold request abuse | Validate patron login before allowing holds; rate limit hold requests |
| CP6 | SSE data exposure | Event stream must not include patron PII — book ID and availability only |
| CP7 | Zip Slip (path traversal) | Validate every file path in uploaded ZIP before extracting; reject `../` paths |
| CP7 | Malicious ZIP import | Validate DB schema after import before bringing app back online |
| CP8 | HTTP security headers | Add middleware for `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` |
| CP8 | Gin proxy warning | Configure `router.SetTrustedProxies([]string{"127.0.0.1"})` for EC2/nginx setup |
| CP8 | HTTPS | Optional — requires a domain name; Let's Encrypt does not issue certs for bare IP addresses. HTTP-only is acceptable for this class deployment. |
| CP8 | Dependency audit | Run `go mod verify` and check for known CVEs before final deploy |

### Session hijacking — design (implemented in CP2)
LibreShelf uses server-side sessions with secure cookies.

- Session tokens generated with `crypto/rand` (cryptographically secure)
- Cookie attributes: `HttpOnly`, `SameSite=Strict`; `Secure` flag is environment-aware (disabled for HTTP-only deployments)
- Session token regenerated after login (prevents session fixation)
- Sessions stored in the `sessions` DB table — can be invalidated server-side
- Short expiry (8 hours) with no sliding renewal — re-login required
- CSRF tokens on all state-changing forms (`POST`, `PUT`, `DELETE`)

### Sensitive data at rest
- `data/database.sqlite` is gitignored — never committed to the repo
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
The app is medium-sized — sub-packages would add indirection without benefit.

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
`modernc.org/sqlite` registers as driver `"sqlite"` (not `"sqlite3"`). Pure Go —
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
