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

**Current Sprint:** Week 7/8 — LibreShelf CP3

**Project history:**
- ✅ Milestone 1: Hello World App ([Issue #1](https://github.com/timLP79/cs408-go-stack/issues/1)) — Gin server, template layout, Bootstrap
- ✅ Testing Infrastructure ([Issue #8](https://github.com/timLP79/cs408-go-stack/issues/8)) — `main_test.go`, httptest, debugging docs
- ✅ Deployment ([Issue #16](https://github.com/timLP79/cs408-go-stack/issues/16)) — EC2, systemd, nginx reverse proxy
- ✅ **Project pivot to LibreShelf** (Week 7) — todo-app issues closed; LibreShelf CPs created
- ✅ **CP1 complete** ([Issue #18](https://github.com/timLP79/cs408-go-stack/issues/18)) — skeleton deployed to EC2
- ✅ **CP2 complete** ([Issue #25](https://github.com/timLP79/cs408-go-stack/issues/25)) — authentication merged, tagged v2

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

**Next up:** [CP3 #19](https://github.com/timLP79/cs408-go-stack/issues/19) — book catalog list and detail pages

**Open milestones:**
- 🔄 [CP3 #19](https://github.com/timLP79/cs408-go-stack/issues/19) — Book catalog: list and detail pages
- [CP4 #20](https://github.com/timLP79/cs408-go-stack/issues/20) — Book CRUD and Open Library API
- [CP5 #21](https://github.com/timLP79/cs408-go-stack/issues/21) — Patron management
- [CP6 #22](https://github.com/timLP79/cs408-go-stack/issues/22) — Loan system: kiosk + SSE
- [CP7 #23](https://github.com/timLP79/cs408-go-stack/issues/23) — Admin panel: ZIP export/import
- [CP8 #24](https://github.com/timLP79/cs408-go-stack/issues/24) — Testing, polish, deploy

---

## Documentation Directory

| File | Description |
|------|-------------|
| [`plan.md`](./plan.md) | LibreShelf architecture, schema, routes, and checkpoint plan |
| [`security.md`](./security.md) | Security model, threat mitigations, and per-CP security checklist |
| [`week7/LibreShelf - Product Specification.pdf`](./week7/LibreShelf%20-%20Product%20Specification.pdf) | LibreShelf product specification (Week 7 assignment) |
| [`week7/wire-frames/`](./week7/wire-frames/) | UI wire frames for all 6 pages |
| [`week6/deployment.md`](./week6/deployment.md) | EC2 deployment guide (systemd + nginx) |
| [`tutorials/GO_LEARNING_GUIDE.md`](./tutorials/GO_LEARNING_GUIDE.md) | Go syntax reference |
| [`week3/BOOTSTRAP_INTEGRATION_GUIDE.md`](./week3/BOOTSTRAP_INTEGRATION_GUIDE.md) | Bootstrap 5 integration guide |
| [`week3/TESTING_AND_DEBUGGING_GUIDE.md`](./week3/TESTING_AND_DEBUGGING_GUIDE.md) | Testing and debugging tutorial |
| [`week3/tech-stack-survey.md`](./week3/tech-stack-survey.md) | Tech stack comparison and rationale |
| [`week3/CANVAS_DISCUSSION_POST.md`](./week3/CANVAS_DISCUSSION_POST.md) | Hello World assignment submission |

---

## What I've Learned So Far

**Milestone 1 — Hello World:**
- ✅ Set up Go module with `go.mod` and dependency management
- ✅ Configured Gin web framework for HTTP routing
- ✅ Implemented Go template rendering with layout pattern
- ✅ Integrated Bootstrap 5 via CDN for styling
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

| CP | Issue | Goal |
|----|-------|------|
| CP1 ✅ | [#18](https://github.com/timLP79/cs408-go-stack/issues/18) | Project skeleton — all 6 routes, nav, DB schema |
| CP2 ✅ | [#25](https://github.com/timLP79/cs408-go-stack/issues/25) | Authentication — login, sessions, role-based access |
| CP3 🔄 | [#19](https://github.com/timLP79/cs408-go-stack/issues/19) | Book catalog and detail pages |
| CP4 | [#20](https://github.com/timLP79/cs408-go-stack/issues/20) | Book CRUD + Open Library API lookup |
| CP5 | [#21](https://github.com/timLP79/cs408-go-stack/issues/21) | Patron management |
| CP6 | [#22](https://github.com/timLP79/cs408-go-stack/issues/22) | Loans, kiosk, and SSE availability |
| CP7 | [#23](https://github.com/timLP79/cs408-go-stack/issues/23) | Admin panel: ZIP export/import |
| CP8 | [#24](https://github.com/timLP79/cs408-go-stack/issues/24) | Testing, polish, final deploy |

---

## Project Structure

```
go-full-stack/
├── main.go                          # Entry point: router, templates, middleware, server ✅
├── main_test.go                     # HTTP handler tests (update pending)
├── db.go                            # DatabaseManager: schema + CRUD methods ✅
├── handlers.go                      # Stub handlers for all 6 pages ✅
├── handlers_books.go                # Book handlers (CP2/CP3)
├── handlers_patrons.go              # Patron handlers (CP4)
├── handlers_loans.go                # Loan/kiosk handlers + SSE (CP5)
├── handlers_admin.go                # Admin handlers: ZIP export/import (CP6)
├── templates/
│   ├── layout.html                  # Base layout with nav bar ✅
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
│   │   └── app.js                   # SSE listener + client JS (CP5)
│   └── images/                      # Cover images, favicon
├── data/                            # SQLite database (gitignored) ✅
├── screenshots/                     # Project screenshots for documentation
├── scripts/
│   ├── install.sh                   # EC2 install script (CP7)
│   └── configure.sh                 # EC2 configure script (CP7)
├── deploy/
│   └── go-full-stack.service        # systemd unit file
├── docs/
│   ├── README.md                    # This file
│   ├── plan.md                      # LibreShelf architecture and checkpoint plan
│   ├── tutorials/GO_LEARNING_GUIDE.md
│   ├── week3/
│   │   ├── BOOTSTRAP_INTEGRATION_GUIDE.md
│   │   ├── TESTING_AND_DEBUGGING_GUIDE.md
│   │   ├── tech-stack-survey.md
│   │   └── CANVAS_DISCUSSION_POST.md
│   ├── week6/
│   │   └── deployment.md
│   └── week7/
│       ├── LibreShelf - Product Specification.pdf
│       └── wire-frames/
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
| `GO_ENV` | (none) | Set to `test` to enable test utilities |

---

## Database

LibreShelf uses a 5-table SQLite schema. The database file is created automatically at startup in the `data/` directory.

### Schema

```sql
CREATE TABLE books (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    isbn TEXT,
    cover_url TEXT,
    published_year INTEGER,
    available INTEGER DEFAULT 1
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

## Routes

| Method | Path | Page | Description |
|--------|------|------|-------------|
| GET | `/` | Dashboard | Stats, recent activity |
| GET | `/catalog` | Catalog | Searchable/filterable book list |
| GET | `/books/:id` | Book Detail | Book info, availability, loan history |
| GET | `/patrons` | Patrons | Patron management |
| GET | `/admin` | Admin | ZIP export/import, settings |
| GET | `/kiosk` | Kiosk | Public browse; optional login for favorites and holds |
| GET | `/events` | SSE | Availability updates stream (CP5) |
| GET | `/stylesheets/*` | — | CSS static files |
| GET | `/javascripts/*` | — | JS static files |
| GET | `/images/*` | — | Image static files |
| GET | `/favicon.svg` | — | Favicon |

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

See [TESTING_AND_DEBUGGING_GUIDE.md](./week3/TESTING_AND_DEBUGGING_GUIDE.md) for a complete tutorial.

---

## Debugging Tools

1. **Print debugging** — `fmt.Println()` in code; `t.Logf()` in tests (only shows with `-v`)
2. **Delve** — official Go debugger: `dlv debug` or `dlv test`
3. **VS Code** — Go extension with Delve integration; switched from GoLand due to resource constraints

See [TESTING_AND_DEBUGGING_GUIDE.md](./week3/TESTING_AND_DEBUGGING_GUIDE.md) for examples.

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

- Full guide: [docs/week6/deployment.md](./week6/deployment.md)
- systemd unit: [`deploy/go-full-stack.service`](../deploy/go-full-stack.service)
- Architecture: Browser → nginx (port 80) → Go app (port 3000)

---

## Learning Resources

### Project docs
- [plan.md](./plan.md) — LibreShelf architecture and design decisions
- [GO_LEARNING_GUIDE.md](./tutorials/GO_LEARNING_GUIDE.md) — Go syntax reference
- [TESTING_AND_DEBUGGING_GUIDE.md](./week3/TESTING_AND_DEBUGGING_GUIDE.md) — Testing tutorial
- [BOOTSTRAP_INTEGRATION_GUIDE.md](./week3/BOOTSTRAP_INTEGRATION_GUIDE.md) — Bootstrap guide
- [tech-stack-survey.md](./week3/tech-stack-survey.md) — Tech stack comparison

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
