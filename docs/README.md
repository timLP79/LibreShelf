# LibreShelf — Project Documentation

A self-hostable library management system built with Go.

**CS408 Spring 2026 Project** | [GitHub Issues](https://github.com/timLP79/cs408-go-stack/issues) | [Project Board](https://github.com/timLP79/cs408-go-stack/projects)

---

## Technology Stack

- **Language**: Go 1.24+
- **Web Framework**: [Gin](https://github.com/gin-gonic/gin)
- **Database**: SQLite (via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) — pure Go, no CGo)
- **Templating**: Go `html/template` with layout pattern
- **CSS**: Bootstrap 5.3 (served locally — no CDN dependency)
- **Deployment**: EC2 + systemd + nginx

---

## Team Workflow

This is a solo developer project.

**Developer:** Tim ([@timLP79](https://github.com/timLP79))

### Workflow
- **GitHub Issues** — every feature, bug, and task is tracked as an issue with labels and milestones
- **GitHub Projects (Kanban)** — issues are organized on a board: Backlog → In Progress → Done
- **Feature Branches** — new work is done on a branch (e.g. `feature/book-catalog`), then merged to `main` via pull request
- **CI on merge** — GitHub Actions runs all tests automatically on every push to `main`
- **Commit references** — commits use `Closes #N` syntax to auto-close issues on merge

---

## Project Status

**Current Sprint:** LibreShelf CP5 complete (landed 6 days ahead of the 4/24 target); CP6 next.

**Project history:**
- ✅ Milestone 1: Hello World App ([Issue #1](https://github.com/timLP79/cs408-go-stack/issues/1)) — Gin server, template layout, Bootstrap
- ✅ Testing Infrastructure ([Issue #8](https://github.com/timLP79/cs408-go-stack/issues/8)) — `main_test.go`, httptest, debugging docs
- ✅ Deployment ([Issue #16](https://github.com/timLP79/cs408-go-stack/issues/16)) — EC2, systemd, nginx reverse proxy
- ✅ **Project pivot to LibreShelf** (Week 7) — todo-app issues closed; LibreShelf CPs created
- ✅ **CP1 complete** ([Issue #18](https://github.com/timLP79/cs408-go-stack/issues/18)) — skeleton deployed to EC2
- ✅ **CP2 complete** ([Issue #25](https://github.com/timLP79/cs408-go-stack/issues/25)) — authentication merged, tagged v2
- ✅ **CP3 complete** ([Issue #19](https://github.com/timLP79/cs408-go-stack/issues/19)): book catalog with search/filter, book detail with loan history, responsive sidebar, bug fixes #28/#29/#30
- ✅ **CP4 complete**: security hardening, three-role model (#38), template error handling (#31), constant-time login (#33), session-bound CSRF protection (#32), `lang="en"` accessibility (#34)
- ✅ **CP5 complete** ([#35](https://github.com/timLP79/cs408-go-stack/issues/35), [#39](https://github.com/timLP79/cs408-go-stack/issues/39), [#20](https://github.com/timLP79/cs408-go-stack/issues/20), [#21](https://github.com/timLP79/cs408-go-stack/issues/21)): full CRUD for staff, books, and patrons. Test router mirror fix (#35), staff management with flash-cookie PRG messaging and live-validation modals (#39), book CRUD with Open Library proxy + cover upload/download validation + Variant B two-button submit (#20, DEC-023), patron CRUD with transactional `CreatePatron` and username auto-generation (#21). 93 tests passing on `cp5-crud`, 6 days ahead of the 4/24 target.

**CP1 — complete:**
- ✅ All 6 template stubs created
- ✅ `layout.html` — nav bar, Bootstrap served locally (offline-ready)
- ✅ `index.html` — Dashboard placeholder with stat cards
- ✅ `db.go` — `DatabaseManager`, 5-table schema created on startup
- ✅ `handlers.go` — stub handlers, `DatabaseMiddleware`, `renderTemplate`
- ✅ `main.go` — all 6 routes, static file serving, DB middleware
- ✅ All routes return 200, schema verified in SQLite
- ✅ `main_test.go` — 3 real tests: index, all routes 200, 404 handler
- ✅ Deployed to EC2 (URL available on request)

**CP2 — complete:**
- ✅ `users` and `sessions` tables added to schema in `db.go`
- ✅ DB methods: `GetUserByUsername`, `CreateUser`, `CreateSession`, `GetSession`, `DeleteSession`
- ✅ WAL mode enabled (`PRAGMA journal_mode=WAL`)
- ✅ `SeedDefaultUsers()` — seeds admin and patron1 on first run
- ✅ `handlers_auth.go` — `generateSessionToken`, `RequireAuth`, `RequireAdmin`, `LoadUser`, `HandleLogin` (GET+POST), `HandleLogout`
- ✅ `RequireAdmin` inlined session check (fixed premature `c.Next()` bug)
- ✅ `templates/login.html` — standalone login page (no layout/sidebar)
- ✅ `templates/error.html` — layout-aware error page for 403/404
- ✅ `renderPage` helper for standalone templates (login, 404)
- ✅ `layout.html` — shows username, Sign Out button, hides admin nav for patrons
- ✅ `index.html` — hides Patrons stat card for patron role
- ✅ Color scheme — slate blue sidebar, soft white-gray background, accent stat cards
- ✅ Merged PR #27, tagged v2

**Next up:** CP6 — loan foundation + kiosk anonymous browse + catalog pagination. Scope disciplined on 2026-04-19 via the v2 reality-check; workflow polish (rapid-scan portal, sidebar restructure, dashboard redesign, printed overdue notices, patron-facing mini-lists) moved to post-submission. Full plan in `cp6-planning.md`.

**Open milestones:**
- [CP6 #22](https://github.com/timLP79/cs408-go-stack/issues/22) Loan system (trimmed): loan schema + transactional DB methods + checkout/return handlers on book-detail scaffold + `/loans` list view with `active|overdue` filter + kiosk anonymous browse. Favorites only if time permits. SSE and patron holds deferred.
- [CP6 #37](https://github.com/timLP79/cs408-go-stack/issues/37) Server-side pagination and filtering for catalog (urgent after CP5 seed bumped the catalog to 100 books)
- CP6 dashboard placeholder wire-up: the three `—` stubs in `templates/index.html` become real counts; no card restructure in CP6
- [CP7 #23](https://github.com/timLP79/cs408-go-stack/issues/23) Admin panel: ZIP export and import (Zip Slip protection) + CSV book/patron import (absorbed from CP6 v2 trim)
- [CP7 #24](https://github.com/timLP79/cs408-go-stack/issues/24) Testing, polish, final EC2 redeploy
- Backlog: [#17](https://github.com/timLP79/cs408-go-stack/issues/17) Automate deployment via GitHub Actions (low priority)

**Post-submission backlog** (items cut from the 4/24-5/1 calendar; see CLAUDE.md for full context):
- CSV patron import (from #21)
- Patron reset-password handler + modal (from #21)
- Patron metadata column UI (from #21)
- Patron activate / deactivate (raised during #21 smoke test)
- SSE live availability updates (from #22)
- Patron holds on checked-out books (from #22)
- Staff table responsive polish (from #39)
- Password-reset Variant 2, server-generated temp + force-change-on-next-login (from #39)
- Orphan cover cleanup on post-cover-save validation failure (from #20)
- Offline detection for the Open Library Lookup button (from #20)

---

## Documentation Directory

| File | Description |
|------|-------------|
| [`plan.md`](./plan.md) | LibreShelf architecture, schema, routes, and checkpoint plan |
| [`security.md`](./security.md) | Security model, threat mitigations, and per-CP security checklist |
| [`product-spec/libreshelf-product-specification.pdf`](./product-spec/libreshelf-product-specification.pdf) | LibreShelf product specification (Week 7 assignment) |
| [`product-spec/wireframes/`](./product-spec/wireframes/) | UI wireframes for all 6 pages |
| [`deployment.md`](./deployment.md) | EC2 deployment guide (systemd + nginx) |
| [`tutorials/go-learning-guide.md`](./tutorials/go-learning-guide.md) | Go syntax reference |
| [`bootstrap-integration-guide.md`](./bootstrap-integration-guide.md) | Bootstrap 5 integration guide |
| [`testing-and-debugging-guide.md`](./testing-and-debugging-guide.md) | Testing and debugging tutorial |
| [`tech-stack-survey.md`](./tech-stack-survey.md) | Tech stack comparison and rationale |
| [`canvas-discussion-post.md`](./canvas-discussion-post.md) | Hello World assignment submission |

---

## What I've Learned So Far

**Milestone 1 — Hello World:**
- ✅ Set up Go module with `go.mod` and dependency management
- ✅ Configured Gin web framework for HTTP routing
- ✅ Implemented Go template rendering with layout pattern
- ✅ Integrated Bootstrap 5 served locally (no CDN dependency)
- ✅ Created clean project structure following Go conventions
- ✅ Learned Git workflow with issue tracking and `Closes #N` syntax

**Testing Infrastructure:**
- ✅ Wrote first test using Go's `testing` package
- ✅ Learned `httptest` for testing HTTP handlers
- ✅ Documented debugging approaches (print debugging, Delve, GoLand)

**CP1 — LibreShelf Skeleton:**
- ✅ Structs and methods with receivers (`type DatabaseManager struct`, `func (dm *DatabaseManager) ...`)
- ✅ Constructor pattern (`NewDatabaseManager`)
- ✅ Go error handling idiom (`if err := ...; err != nil { log.Fatalf(...) }`)
- ✅ Side-effect imports (`import _ "modernc.org/sqlite"`)
- ✅ Type assertions (`c.MustGet("db").(*DatabaseManager)`)
- ✅ Closures and middleware factories (`DatabaseMiddleware` returns `gin.HandlerFunc`)
- ✅ Range loops over slices (`for _, name := range templateNames`)
- ✅ Environment variables (`os.Getenv` with fallback defaults)
- ✅ URL parameters (`c.Param("id")`)
- ✅ False positives in testing and how to avoid them
- ✅ `t.Helper()`, `t.Cleanup()`, and `os.MkdirTemp` for isolated test databases
- ✅ Static file serving locally (offline Bootstrap — no CDN dependency)
- ✅ Many-to-many database relationships (junction table pattern)
- ✅ Deploying: stop service before `scp`, `git pull` for templates

**Key Go Concepts Mastered:**
- Package structure and imports
- Gin router setup (`gin.Default()`, `router.GET()`)
- Template execution (`ExecuteTemplate()`)
- Go template syntax (`{{define}}`, `{{block}}`, `{{.Variable}}`)
- Struct types and data passing to templates
- Environment configuration with `os.Getenv()`
- Testing with `testing` package and `httptest`

**Development Tools:**
- `go run .` — run without building
- `go build` — compile to executable
- `go mod tidy` — sync go.sum with actual dependencies
- `go test -v` — run tests with verbose output
- Git commit messages with issue references
- VS Code with Go extension (switched from GoLand — resource constraints)
- Delve debugger for advanced debugging

---

## Checkpoint Plan

See [`plan.md`](./plan.md) for the full LibreShelf architecture. Summary:

| CP | Status | Goal |
|----|--------|------|
| CP1 | ✅ | Project skeleton: all routes, nav, DB schema ([#18](https://github.com/timLP79/cs408-go-stack/issues/18)) |
| CP2 | ✅ | Authentication: login, sessions, role-based access ([#25](https://github.com/timLP79/cs408-go-stack/issues/25)) |
| CP3 | ✅ | Book catalog and detail pages ([#19](https://github.com/timLP79/cs408-go-stack/issues/19)) |
| CP4 | ✅ | Security hardening: three-role model, CSRF, constant-time login, ExecuteTemplate fix |
| CP5 | ✅ | CRUD: book CRUD + Open Library, patron management, staff management, test router mirror fix ([#20](https://github.com/timLP79/cs408-go-stack/issues/20), [#21](https://github.com/timLP79/cs408-go-stack/issues/21), [#39](https://github.com/timLP79/cs408-go-stack/issues/39), [#35](https://github.com/timLP79/cs408-go-stack/issues/35)) |
| CP6 | 🔄 | Loan foundation (schema, transactional checkout/return, `/loans` list view), server-side pagination (#37), kiosk anonymous browse, dashboard placeholder wire-up. Portal, sidebar restructure, dashboard redesign, overdue notice print system deferred post-submission per the 2026-04-19 v2 trim. ([#22](https://github.com/timLP79/cs408-go-stack/issues/22), [#37](https://github.com/timLP79/cs408-go-stack/issues/37)) |
| CP7 | | Admin panel (ZIP export/import + CSV book/patron import absorbed from CP6), security headers, final polish and deploy ([#23](https://github.com/timLP79/cs408-go-stack/issues/23), [#24](https://github.com/timLP79/cs408-go-stack/issues/24)) |

---

## Project Structure

```
go-full-stack/
├── main.go                          # Entry point: router, templates, middleware, server ✅
├── main_test.go                     # HTTP handler tests (update pending)
├── db.go                            # DatabaseManager: schema + CRUD methods ✅
├── handlers.go                      # Stub handlers for all 6 pages ✅
├── handlers_auth.go                 # Login, logout, session middleware ✅
├── handlers_books.go                # Book handlers (CP3/CP4)
├── handlers_patrons.go              # Patron handlers (CP5)
├── handlers_loans.go                # Checkout/return + /loans list view (CP6; SSE deferred post-submission)
├── handlers_admin.go                # Admin handlers: ZIP export/import, Open Library proxy (CP4/CP7)
├── templates/
│   ├── layout.html                  # Base layout with sidebar nav ✅
│   ├── login.html                   # Standalone login page (no layout) ✅
│   ├── index.html                   # Dashboard page ✅
│   ├── catalog.html                 # Book catalog placeholder ✅
│   ├── book_detail.html             # Book detail placeholder ✅
│   ├── patrons.html                 # Patrons placeholder ✅
│   ├── admin.html                   # Admin placeholder ✅
│   ├── kiosk.html                   # Kiosk placeholder ✅
│   └── error.html                   # 404/500 error page ✅
├── static/
│   ├── stylesheets/
│   │   ├── bootstrap.min.css        # Bootstrap 5.3 (local, offline-ready) ✅
│   │   └── style.css                # Custom styles ✅
│   ├── javascripts/
│   │   ├── bootstrap.bundle.min.js  # Bootstrap JS (local) ✅
│   │   └── app.js                   # Catalog filter, Bootstrap live validation, staff/patron/book management, OL Lookup wiring (CP3-CP5) ✅
│   └── images/                      # Cover images, favicon
├── data/                            # SQLite database (gitignored) ✅
├── screenshots/                     # Project screenshots for documentation
├── scripts/
│   ├── install.sh                   # EC2 install script (CP7)
│   └── configure.sh                 # EC2 configure script (CP7)
├── deploy/
│   └── go-full-stack.service        # systemd unit file
├── DECISIONS.md                         # Design decisions log
├── docs/
│   ├── README.md                    # This file
│   ├── plan.md                      # LibreShelf architecture and checkpoint plan
│   ├── security.md                  # Security model and per-CP mitigations
│   ├── deployment.md                # EC2 deployment guide (systemd + nginx)
│   ├── cp6-planning.md              # CP6 planning notes + deferred design
│   ├── bootstrap-integration-guide.md
│   ├── testing-and-debugging-guide.md
│   ├── tech-stack-survey.md
│   ├── canvas-discussion-post.md
│   ├── tutorials/
│   │   └── go-learning-guide.md
│   └── product-spec/
│       ├── libreshelf-product-specification.pdf
│       └── wireframes/
├── go.mod
├── go.sum
└── README.md                        # Project intro and quick start
```

---

## Getting Started

### Prerequisites

- Go 1.24 or higher
- Git

### Installation

```bash
git clone https://github.com/timLP79/cs408-go-stack.git
cd cs408-go-stack
go mod download
go run .
```

Visit `http://localhost:3000` in your browser.

### Development

**Quick run:**
```bash
go run .
```

**Build and run:**
```bash
go build -o go-full-stack .
./go-full-stack
```

**Live reload with air:**
```bash
go install github.com/cosmtrek/air@latest
air
```

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | HTTP server port |
| `DATA_DIR` | `data` | Directory for SQLite database |
| `DB_NAME` | `database.sqlite` | Database filename |
| `ADMIN_PASSWORD` | `Admin123!` | Override default admin password. Must pass `ValidatePassword` (8+ chars, uppercase, digit, special) or startup fails (DEC-021). |
| `APP_ENV` | (none) | Set to `production` to enable Secure cookie flag |

---

## Database

LibreShelf uses a 7-table SQLite schema. The database file is created automatically at startup in the `data/` directory. See [`plan.md`](./plan.md) for the authoritative schema; the snippet below is a convenience copy.

### Schema

```sql
CREATE TABLE books (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    title              TEXT NOT NULL,
    isbn               TEXT UNIQUE,
    cover_filename     TEXT,
    year               INTEGER,
    publisher          TEXT,
    description        TEXT,
    genre              TEXT,
    quantity_total     INTEGER DEFAULT 1,
    quantity_available INTEGER DEFAULT 1
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
    joined_date DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata    TEXT  -- JSON, nullable, added in CP5 (DEC-016)
);

CREATE TABLE loans (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id        INTEGER REFERENCES books(id),
    patron_id      INTEGER REFERENCES patrons(id),
    checked_out_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    due_date       DATETIME,
    returned_at    DATETIME
);

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK(role IN('admin', 'staff', 'patron')),
    patron_id     INTEGER REFERENCES patrons(id),  -- NULL for admin and staff
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    csrf_token TEXT NOT NULL,  -- bound to session (CP4, DEC-017)
    expires_at DATETIME NOT NULL  -- canonical UTC "YYYY-MM-DD HH:MM:SS" (DEC-018)
);
```

---

## Routes

| Method | Path | Page | Access | Description |
|--------|------|------|--------|-------------|
| GET | `/login` | Login | Public | Login page |
| POST | `/login` | Login | Public | Login action |
| POST | `/logout` | — | Any logged-in | Logout action |
| GET | `/` | Dashboard | RequireAuth | Stats, recent activity |
| GET | `/catalog` | Catalog | RequireAuth | Searchable/filterable book list |
| GET | `/books/:id` | Book Detail | RequireAuth | Book info, availability, loan history |
| GET | `/patrons` | Patrons | RequireAdmin | Patron management |
| GET | `/admin` | Admin | RequireAdmin | ZIP export/import, settings |
| GET | `/kiosk` | Kiosk | Public | Anonymous public browse (CP6 scope); favorites-if-time-permits requires patron login |
| GET | `/loans` | Loan list view | RequireStaff | Active/overdue loans with filter; each row has a Return action *(planned, CP6)* |
| POST | `/loans/checkout` | — | RequireStaff | Checkout action from book-detail scaffold *(planned, CP6)* |
| POST | `/loans/:id/return` | — | RequireStaff | Return action from /loans or book detail *(planned, CP6)* |
| GET | `/events` | SSE | RequireAuth | Availability updates stream *(deferred post-submission)* |
| GET | `/stylesheets/*` | — | Public | CSS static files |
| GET | `/javascripts/*` | — | Public | JS static files |
| GET | `/images/*` | — | Public | Image static files |

---

## Templates

| Template | Description |
|----------|-------------|
| `layout.html` | Base layout with sidebar navigation (Bootstrap flexbox, local — no CDN) |
| `index.html` | Dashboard — stats and recent activity |
| `catalog.html` | Book catalog with search/filter |
| `book_detail.html` | Single book view with loan history |
| `patrons.html` | Patron list and management |
| `admin.html` | Admin panel |
| `kiosk.html` | Public catalog browse (optional patron login for favorites and holds) |
| `error.html` | 404/500 error page |

### Template Syntax

| Syntax | Description |
|--------|-------------|
| `{{.Title}}` | Output a variable |
| `{{define "name"}}...{{end}}` | Define a template block |
| `{{block "name" .}}{{end}}` | Insert a block (with fallback) |
| `{{template "name" .}}` | Include another template |

---

## Testing

### Strategy

1. **Unit tests** — individual DB methods (`db_test.go`)
2. **Integration tests** — HTTP handlers with real requests (`main_test.go`, `handlers_test.go`)

### Running Tests

```bash
go test ./...           # run all tests
go test -v ./...        # verbose output
go test -cover ./...    # coverage report
```

### Test File Convention

- `main.go` → `main_test.go`
- `db.go` → `db_test.go`
- `handlers.go` → `handlers_test.go`

### Basic Test Structure

```go
func TestSomething(t *testing.T) {
    // Arrange
    expected := "expected value"

    // Act
    result := YourFunction()

    // Assert
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

See [testing-and-debugging-guide.md](./testing-and-debugging-guide.md) for a complete tutorial.

---

## Debugging Tools

1. **Print debugging** — `fmt.Println()` in code; `t.Logf()` in tests (only shows with `-v`)
2. **Delve** — official Go debugger: `dlv debug` or `dlv test`
3. **VS Code** — Go extension with Delve integration; switched from GoLand due to resource constraints

See [testing-and-debugging-guide.md](./testing-and-debugging-guide.md) for examples.

---

## GitHub Labels

| Label | Use |
|-------|-----|
| `milestone` | Major deliverable |
| `backend` | Server-side logic, handlers, API |
| `frontend` | UI, templates, client-side |
| `database` | Schema, migrations, DB methods |
| `testing` | Test-related work |
| `priority-high` | Do this soon |
| `priority-low` | Can wait |
| `learning` | Educational value |
| `blocked` | Waiting on a dependency |

---

## Git Workflow

1. Pick an issue from the [GitHub Issues board](https://github.com/timLP79/cs408-go-stack/issues)
2. Make changes on a branch or directly on `main` for small fixes
3. Test locally: `go run .` then visit `http://localhost:3000`
4. Commit with issue reference:
   ```bash
   git add <files>
   git commit -m "Brief description

   Closes #<issue-number>"
   ```
5. Push — CI runs automatically; issue closes on merge

### Example commit message

```
Add LibreShelf DB schema and route stubs

Create db.go with 5-table schema (books, authors, book_authors, patrons, loans).
Add stub handlers in handlers.go for all 6 pages. Register routes in main.go.

Closes #18
```

---

## Deployment

The app runs as a systemd service behind an nginx reverse proxy on Ubuntu EC2.

- Full guide: [docs/deployment.md](./deployment.md)
- systemd unit: [`deploy/go-full-stack.service`](../deploy/go-full-stack.service)
- Architecture: Browser → nginx (port 80) → Go app (port 3000)

---

## Learning Resources

### Project docs
- [plan.md](./plan.md) — LibreShelf architecture and design decisions
- [go-learning-guide.md](./tutorials/go-learning-guide.md) — Go syntax reference
- [testing-and-debugging-guide.md](./testing-and-debugging-guide.md) — Testing tutorial
- [bootstrap-integration-guide.md](./bootstrap-integration-guide.md) — Bootstrap guide
- [tech-stack-survey.md](./tech-stack-survey.md) — Tech stack comparison

### External
- [Gin Documentation](https://gin-gonic.com/docs/)
- [Go Templates](https://pkg.go.dev/html/template)
- [Go Testing](https://pkg.go.dev/testing)
- [Tour of Go](https://go.dev/tour/)
- [Open Library API](https://openlibrary.org/dev/docs/api)

---

## License

(To be determined)

## Acknowledgments

- Original starter: [Full Stack Starter](https://github.com/shanep/fullstack-starter) (Node.js)
- Built as a learning exercise for CS408 Go web development
