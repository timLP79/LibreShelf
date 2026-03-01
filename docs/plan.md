# Plan: LibreShelf — Library Management System

## Overview

LibreShelf is a self-hostable library management system built for CS408 Spring 2026.
It replaces the earlier Hello World / todo-app demo and uses the same proven tech stack
and deployment infrastructure.

LibreShelf lets a small library (school, office, personal collection) manage books,
patrons, and loans through a simple web UI. A kiosk mode allows self-service check-in
and check-out with real-time availability updates.

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

| Route | Page | Description |
|-------|------|-------------|
| `GET /` | Dashboard | Stats, recent activity, quick-add book |
| `GET /catalog` | Catalog | Searchable/filterable book list |
| `GET /books/:id` | Book Detail | Book info, availability, loan history |
| `GET /patrons` | Patrons | Patron list and management |
| `GET /admin` | Admin | Settings, ZIP export/import |
| `GET /kiosk` | Kiosk | Self-service check-in / check-out |

---

## Database Schema

```sql
CREATE TABLE books (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    isbn TEXT,
    cover_url TEXT,
    published_year INTEGER,
    available INTEGER DEFAULT 1  -- boolean: 0 or 1
);

CREATE TABLE authors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL
);

CREATE TABLE book_authors (
    book_id INTEGER REFERENCES books(id),
    author_id INTEGER REFERENCES authors(id),
    PRIMARY KEY (book_id, author_id)
);

CREATE TABLE patrons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT
);

CREATE TABLE loans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INTEGER REFERENCES books(id),
    patron_id INTEGER REFERENCES patrons(id),
    checked_out_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    due_date DATETIME,
    returned_at DATETIME
);
```

---

## Directory Structure

```
go-full-stack/
├── main.go                    # Entry point: router, template loading, middleware, server
├── db.go                      # DatabaseManager: schema creation + all CRUD methods
├── handlers.go                # HTTP handler functions for all 6 pages
├── handlers_books.go          # Book-specific handlers (catalog, detail, CRUD)
├── handlers_patrons.go        # Patron handlers
├── handlers_loans.go          # Loan/kiosk handlers + SSE endpoint
├── handlers_admin.go          # Admin handlers (ZIP export, import, Open Library proxy)
├── templates/
│   ├── layout.html            # Base layout with nav (Dashboard, Catalog, Patrons, Admin, Kiosk)
│   ├── index.html             # Dashboard page
│   ├── catalog.html           # Book catalog list
│   ├── book_detail.html       # Single book view
│   ├── patrons.html           # Patron list
│   ├── admin.html             # Admin panel
│   ├── kiosk.html             # Kiosk check-in/out
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
- ✅ Deployed to EC2 at `http://18.237.205.236`

---

### CP2 — Book Catalog & Detail Pages
**Goal:** `/catalog` shows real books from DB; `/books/:id` shows full book detail.

- `handlers_books.go`: `HandleCatalog`, `HandleBookDetail`
- `db.go`: `GetAllBooks()`, `GetBookByID()`
- `templates/catalog.html`: searchable/filterable book list
- `templates/book_detail.html`: book info, availability, loan history

---

### CP3 — Book CRUD + Open Library API
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

### CP4 — Patron Management
**Goal:** `/patrons` shows patron list with full CRUD.

- `handlers_patrons.go`: list, add, edit, delete
- `db.go`: `GetAllPatrons()`, `GetPatronByID()`, `CreatePatron()`, `UpdatePatron()`, `DeletePatron()`
- `templates/patrons.html`: patron list with inline forms

---

### CP5 — Loans & Kiosk + SSE
**Goal:** Kiosk enables self-service check-in/out. SSE pushes live availability updates to the Catalog.

- `handlers_loans.go`: `HandleKiosk`, `POST /loans`, `PUT /loans/:id/return`, `GET /events`
- `db.go`: `CreateLoan()`, `ReturnLoan()`, `GetActiveLoans()`, `GetLoanHistory()`
- `templates/kiosk.html`: self-service UI

**SSE protocol:**
- Endpoint: `GET /events`
- Message format: `data: book_id=N available=0\n\n`
- Catalog page JS listens and updates availability badges without page reload

---

### CP6 — Admin Panel (ZIP Export/Import)
**Goal:** Admin can export the entire DB as a ZIP and import it back.

- `handlers_admin.go`: `GET /admin/export`, `POST /admin/import`
- `templates/admin.html`: export button, import file picker, system stats
- Uses Go standard library `archive/zip` — no extra dependencies

ZIP contains: SQLite database file + cover images from `static/images/covers/`

---

### CP7 — Testing, Polish & Deploy
**Goal:** Test coverage, UI cleanup, final EC2 redeploy.

- `db_test.go`: unit tests for all DB methods
- `handlers_test.go`: integration tests for all HTTP handlers
- `scripts/install.sh`, `scripts/configure.sh`: EC2 automation scripts

---

## Key Design Decisions

### 1. Flat package structure
All `.go` files in `package main`, split by concern using filename suffix
(`handlers_books.go`, `handlers_patrons.go`, etc.) rather than sub-packages.
The app is medium-sized — sub-packages would add indirection without benefit.

### 2. SSE for real-time availability
On kiosk check-out/in, the server pushes a Server-Sent Events message to all
connected catalog clients. SSE is one-way (server → browser) and fits the use
case exactly. No WebSocket needed.

### 3. Open Library API (server-side proxy)
ISBN metadata is fetched server-side to avoid CORS issues and to keep the client
JS simple. The handler proxies the request and returns clean JSON to the form.

### 4. ZIP export using standard library
`archive/zip` (standard library) is sufficient. No third-party dependency needed
for backup/restore.

### 5. Template loading
Same pattern as the current codebase: `map[string]*template.Template`, one entry
per page, each paired with `layout.html`. The `renderTemplate()` helper executes
the `"layout"` template, which pulls in the page's `"content"` block.

### 6. SQLite driver
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
