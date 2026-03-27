# LibreShelf

A self-hostable library management system built with Go.

**CS408 Spring 2026 Project** | [GitHub Issues](https://github.com/timLP79/cs408-go-stack/issues) | [Project Board](https://github.com/timLP79/cs408-go-stack/projects)

LibreShelf lets a small library (school, office, personal collection) manage books,
patrons, and loans through a simple web UI. A kiosk mode supports self-service
check-in and check-out with real-time availability updates via Server-Sent Events.

## Tech Stack

- **Go 1.24+** with [Gin](https://github.com/gin-gonic/gin) web framework
- **SQLite** via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go, no CGo)
- **Go `html/template`** with layout pattern
- **Bootstrap 5.3** (served locally — no CDN dependency)
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
| `admin` | `admin123` | Admin — full access to all pages |
| `patron1` | `patron123` | Patron — limited access (no admin/patrons pages) |

## Pages

| Route | Page |
|-------|------|
| `/` | Dashboard — stats and recent activity |
| `/catalog` | Book catalog — searchable and filterable |
| `/books/:id` | Book detail — info, availability, loan history |
| `/patrons` | Patron management |
| `/admin` | Admin panel — ZIP export/import |
| `/kiosk` | Public browse — optional login for favorites and holds |

## Documentation

- [Technical plan and architecture](./docs/plan.md)
- [Product specification](./docs/week7/LibreShelf%20-%20Product%20Specification.pdf)
- [UI wire frames](./docs/week7/wire-frames/)
- [Deployment guide (EC2 + systemd + nginx)](./docs/week6/deployment.md)
- [Go learning guide](./docs/tutorials/GO_LEARNING_GUIDE.md)

## License

(To be determined)
