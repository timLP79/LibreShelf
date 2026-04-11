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
**Status:** In development -- CP4 complete, CP5 next

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
- `main.go`, `db.go`, `handlers.go`, `handlers_auth.go`, `handlers_books.go`, `main_test.go`
- All 9 HTML templates, layout with responsive offcanvas sidebar
- `static/javascripts/app.js` (client-side catalog filtering)
- `static/stylesheets/style.css` (custom styles including availability badges, responsive sidebar)

---

## Standards

- Handle all errors explicitly in Go -- never ignore returned errors
- Log errors server-side, return generic messages to clients
- Use environment variables for all secrets -- never hardcode
- Return correct HTTP status codes
- Validate and sanitize inputs server-side on every endpoint
- New endpoints need rate limiting and CORS handling from the start
- Always use parameterized queries (`?` placeholders) -- never string concatenation
- Commits should be descriptive and reference issue numbers where applicable
- Keep solutions lightweight -- consistent with the Absolute Code philosophy

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

### CP3 (complete)
- [x] #19 -- Book catalog: list and detail pages
- [x] #28 -- Fix: `CreateSession` error silently ignored in login handler
- [x] #29 -- Fix: `SeedDefaultUsers` ignores multiple errors
- [x] #30 -- Fix: `renderPage` template name mismatch causes blank 404
- [x] #40 -- Responsive sidebar with offcanvas mobile menu

### CP4 (complete)
- [x] #34 -- Add `lang="en"` to HTML tags (WCAG 2.1)
- [x] #38 -- Three-role model: admin, staff, patron
- [x] #31 -- `ExecuteTemplate` errors never checked in render helpers
- [x] #33 -- Username enumeration via login timing side-channel
- [x] #32 -- CSRF protection via session-bound synchronizer token

### CP5 -- CRUD Features (Books, Patrons, Staff)
- [ ] #20 -- Book CRUD and Open Library API lookup
- [ ] #21 -- Patron management: CRUD, metadata, and CSV import
- [ ] #39 -- Staff management: list, add, edit, delete

### CP6 -- Loans + Kiosk + SSE
- [ ] #22 -- Loan system: kiosk browse, holds, and SSE availability
- [ ] #37 -- Server-side pagination and filtering for catalog

### CP7 -- Admin Panel + Testing + Deploy
- [ ] #23 -- Admin panel: ZIP export and import
- [ ] #35 -- Fix: Test router does not mirror production middleware
- [ ] #24 -- Testing, polish, and deploy

### Backlog
- [ ] #17 -- Automate deployment via GitHub Actions (low priority)
