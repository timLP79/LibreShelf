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

Files that exist:
- `main.go`, `db.go`, `handlers.go`, `handlers_auth.go`, `handlers_books.go`, `validators.go`, `main_test.go`
- All HTML templates including `staff.html` (on branch `cp5-crud`), layout with responsive offcanvas sidebar and admin-only Staff link
- `static/javascripts/app.js` (client-side catalog filtering, plus staff management modal logic on `cp5-crud`)
- `static/stylesheets/style.css` (custom styles including availability badges, responsive sidebar)
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

### CP5 -- CRUD Features (Staff, Books, Patrons) -- in progress on `cp5-crud`

Order: close #39, then #20, then #21.

- [x] #35 -- Fix: Test router does not mirror production middleware (closed in b9ce9ed). `setupTestRouter` mirrors main.go route groups. Added `loginAs` and `logoutHelper` test helpers. Three new regression-pin tests cover the middleware chain.
- [ ] #39 -- Staff management: close out
    - Design locked (see DEC-019, DEC-020). Template, JS, sidebar link, favicon done.
    - `db.go` methods done: `GetAllStaff`, `GetUserByID`, `UpdateStaffUser`, `DeleteUser` (transactional), `CountAdmins`. `CreateUser` is reusable as-is.
    - `validators.go` done: `ValidateUsername`, `ValidatePassword` (see DEC-021).
    - `SeedBooks` retrofitted into `seedOneBook` per-book transaction (see DEC-022).
    - `SeedDefaultUsers` bumped to `Admin123!` / `Staff123!` / `Patron123!` and validates `ADMIN_PASSWORD` at startup.
    - Tests done: `validators_test.go`, `db_test.go`. Full suite 35 passing on `cp5-crud`.
    - Remaining (tutor mode): `handlers_staff.go` (new file), route registration in `main.go`, `handlers_staff_test.go` for the guard rules. Close #35 first so the handler tests exercise the real auth + CSRF middleware chain.
- [ ] #20 -- Book CRUD + Open Library API
    - Handlers, DB methods (transactional: books + authors + book_authors per DEC-022), cover upload with MIME/size/extension validation, Open Library proxy endpoint, tests.
- [ ] #21 -- Patron CRUD (**CSV import deferred post-submission**)
    - Handlers, DB methods (`CreatePatron` transactional: patrons + users per DEC-022), patron list template with modals, auto-generated username, policy-compliant temp password, tests.

### CP6 -- Loans + Kiosk + Pagination

- [ ] #22 -- Loan system: checkout/return + kiosk browse + favorites (**SSE and patron holds deferred post-submission**)
- [ ] #37 -- Server-side pagination and filtering for catalog (needed once Open Library grows the catalog past ~6 books)

### CP7 -- Admin Panel + Security Hardening + Deploy

- [ ] #23 -- Admin panel: ZIP export and import (with Zip Slip protection)
- [ ] #24 -- Testing, polish, and deploy
    - `SecurityHeaders` middleware, `SetTrustedProxies`, `go mod verify`, `govulncheck`, final EC2 redeploy with a clean DB to pick up new seed passwords.

### Deferred post-submission backlog

- CSV patron import (from #21)
- SSE live availability updates (from #22)
- Patron holds on checked-out books (from #22)
- [ ] #17 -- Automate deployment via GitHub Actions (already low-priority backlog)
