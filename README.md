# LibreShelf

A self-hostable library management system built with Go.

**CS408 Spring 2026 Project** | [GitHub Issues](https://github.com/timLP79/cs408-go-stack/issues) | [Project Board](https://github.com/timLP79/cs408-go-stack/projects)

LibreShelf lets a small library (school, office, personal collection) manage books,
patrons, and loans through a simple web UI. A public kiosk supports self-service
browsing with optional patron login for favorites and holds. All checkout and return
transactions are staff-only. (SSE live availability is on the post-submission roadmap.)

## Tech Stack

- **Go 1.24+** with [Gin](https://github.com/gin-gonic/gin) web framework
- **SQLite** via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go, no CGo)
- **Go `html/template`** with layout pattern
- **Bootstrap 5.3** (served locally -- no CDN dependency)
- **EC2 + systemd + nginx** (deployment)

## Quick Start

```bash
git clone https://github.com/timLP79/cs408-go-stack.git
cd cs408-go-stack
go mod download
go run .
```

Visit `http://localhost:3000` in your browser.

## Test Accounts

These accounts are created automatically on first run:

| Username | Password | Role |
|----------|----------|------|
| `admin` | `Admin123!` | Admin -- full access to all pages |
| `staff1` | `Staff123!` | Staff -- day-to-day operations, no staff/system management |
| `patron1` | `Patron123!` | Patron -- catalog and book detail only |

Passwords must be 8+ characters with at least one uppercase letter, one digit, and one special character. See [DEC-021](./DECISIONS.md) for the policy.

## Pages

| Route | Page | Access |
|-------|------|--------|
| `/` | Dashboard -- stats and recent activity | Any logged-in user |
| `/catalog` | Book catalog -- searchable and filterable, 4-wide grid | Any logged-in user |
| `/books/:id` | Book detail -- info, availability, loan history | Any logged-in user |
| `/books/new` | Add-book form with Open Library ISBN lookup | Staff + admin |
| `/books/:id/edit` | Edit-book form | Staff + admin |
| `/patrons` | Patron management -- list + add / edit / delete modals | Staff + admin |
| `/staff` | Staff management -- list + add / edit / delete / reset-password modals | Admin only |
| `/admin` | Admin panel -- stub today; CP7 adds ZIP export / import | Staff + admin (CP7 ZIP features will be admin-only) |
| `/kiosk` | Public browse -- stub today; CP6 adds favorites and patron login (holds deferred post-submission) | Public |

## Documentation

- [Technical plan and architecture](./docs/plan.md)
- [Product specification](./docs/week7/LibreShelf%20-%20Product%20Specification.pdf)
- [UI wire frames](./docs/week7/wire-frames/)
- [Deployment guide (EC2 + systemd + nginx)](./docs/week6/deployment.md)
- [Go learning guide](./docs/tutorials/GO_LEARNING_GUIDE.md)

## License

(To be determined)
