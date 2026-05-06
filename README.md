# LibreShelf

A self-hostable library management system, built in Go.

LibreShelf lets a small library -- a school, office, or personal collection -- manage books,
patrons, and loans through a simple web UI. A public kiosk lets anyone browse the catalog
without logging in. All checkout and return transactions are handled by staff.

## Features

- Book catalog with search, genre filter, and availability filter
- Open Library ISBN lookup with cover image preview on add/edit
- Loan transactions with overdue tracking; due dates derived from `due_date` and `returned_at`, not stored
- Three-role access model: admin, staff, patron
- Public kiosk for anonymous catalog browsing
- ZIP backup export and import with Zip Slip protection and atomic `.bak` rollback
- CSRF protection, session-bound tokens, bcrypt password hashing
- Defensive HTTP headers (CSP, X-Frame-Options, HSTS gated on production)
- Pure-Go SQLite via `modernc.org/sqlite` -- no CGo, no system libraries

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25.9+ |
| Web framework | [Gin](https://github.com/gin-gonic/gin) |
| Templating | Go `html/template` with layout pattern |
| Database | SQLite via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go, no CGo) |
| CSS | Bootstrap 5.3 (served locally; no CDN) |
| Deployment | systemd + nginx |

## Quick Start

```bash
git clone https://github.com/timLP79/LibreShelf.git
cd LibreShelf
go mod download
go run .
```

Visit `http://localhost:3000`. The schema is created on first run, and three default accounts
are seeded (see below).

## Default Accounts

Created on first run if they don't already exist.

| Username | Password | Role |
|----------|----------|------|
| `admin` | `Admin123!` | Admin -- full access |
| `staff1` | `Staff123!` | Staff -- day-to-day operations |
| `patron1` | `Patron123!` | Patron -- catalog and book detail |

Override the admin password by setting the `ADMIN_PASSWORD` environment variable. Passwords
must be 8+ characters with at least one uppercase letter, one digit, and one special character;
the policy is enforced at startup. See [DEC-021](./DECISIONS.md) for the rationale.

## Pages and Access

| Route | Page | Access |
|-------|------|--------|
| `/` | Dashboard with role-differentiated stat cards | Any logged-in user |
| `/catalog` | Searchable book grid | Any logged-in user |
| `/books/:id` | Book detail with availability and loan history | Any logged-in user |
| `/books/new`, `/books/:id/edit` | Add or edit a book, with Open Library lookup | Staff + admin |
| `/patrons` | Patron management with add / edit / delete modals | Staff + admin |
| `/staff` | Staff management with add / edit / delete / reset-password modals | Admin |
| `/admin` | Admin tools index | Admin |
| `/admin/backup` | Library statistics, ZIP export, restore-from-backup modal | Admin |
| `/loans` | Active and overdue loans, with role filter | Staff + admin |
| `/my/loans` | Patron's own active loans | Patron |
| `/kiosk`, `/kiosk/books/:id` | Public anonymous browse | Public |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | HTTP server port |
| `DATA_DIR` | `data` | Directory for SQLite database and cover images |
| `DB_NAME` | `database.sqlite` | Database filename |
| `ADMIN_PASSWORD` | `Admin123!` | Override the seeded admin password. Validated against the password policy at startup |
| `APP_ENV` | (unset) | Set to `production` to enable the `Secure` cookie flag and HSTS |

## Documentation

- [Architecture and design reference](./docs/architecture.md) -- routes, schema, directory layout, design decisions
- [Security model](./docs/security.md) -- threat model, mitigations, auth, CSRF, headers
- [Deployment guide](./docs/deployment.md) -- build, systemd, nginx
- [Product specification](./docs/product-spec/libreshelf-product-specification.pdf) (PDF)
- [UI wireframes](./docs/product-spec/wireframes/) (PDF)
- [Design decisions log](./DECISIONS.md)

## Status

Feature-complete for the original v1 scope. Open follow-up work tracked locally via the beads
tool (run `bd list --status=open`): SSE live availability, favorites, patron holds, CSV patron
import, fuller dashboard with mini-lists, server-side catalog pagination, overdue notice print
system, and a few UX polish items.

## License

LibreShelf is proprietary commercial software. Copyright (c) 2026 Tim Palacios.
All rights reserved. See [LICENSE](./LICENSE) for terms.

The source is published here for portfolio and evaluation purposes only.
Compiling, running, hosting, or redistributing LibreShelf requires a separate
commercial license. To inquire, please contact the author.
