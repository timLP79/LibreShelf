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
**Status:** In development -- CP3 complete, CP4 next

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
**CP3 -- Book Catalog & Detail Pages:** Complete. Catalog with search/filter, book detail with metadata and loan history, bug fixes #28/#29/#30.
**CP4 -- Book CRUD & Open Library API:** Next.

Files that exist:
- `main.go`, `db.go`, `handlers.go`, `handlers_auth.go`, `handlers_books.go`, `main_test.go`
- All 9 HTML templates, layout with sticky sidebar nav
- `static/javascripts/app.js` (client-side catalog filtering)
- `static/stylesheets/style.css` (custom styles including availability badges)

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

### CP4 (next)
- [ ] #20 -- [CP4] Book CRUD and Open Library API lookup
- [ ] #21 -- [CP5] Patron management: list, add, edit, delete
- [ ] #22 -- [CP6] Loan system: kiosk browse, holds, and SSE availability
- [ ] #23 -- [CP7] Admin panel: ZIP export and import
- [ ] #37 -- [CP7] Server-side pagination and filtering for catalog
- [ ] #24 -- [CP8] Testing, polish, and deploy

### Bug fixes (assigned to future CPs)
- [ ] #31 -- [CP4] `ExecuteTemplate` errors never checked in render helpers
- [ ] #32 -- [CP4] CSRF protection not implemented
- [ ] #33 -- [CP5] Username enumeration via login timing side-channel
- [ ] #34 -- [CP5] Missing `lang="en"` on HTML tags (WCAG 2.1)
- [ ] #35 -- [CP8] Test router does not mirror production middleware

### Backlog
- [ ] #17 -- Automate deployment via GitHub Actions (low priority)
