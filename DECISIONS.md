# LibreShelf -- Design Decisions

Architectural and design decisions made during development. Each entry records the context,
the decision, and the reasoning so future sessions start with full understanding.

---

## DEC-001: Bootstrap served locally, not via CDN

**Date:** 2026-02-01 (CP1)
**Context:** The product specification listed "Bootstrap 5 via CDN." The app is designed to be
self-hostable and offline-capable.
**Decision:** Serve Bootstrap from `static/stylesheets/` and `static/javascripts/` instead of a CDN.
**Rationale:** Eliminates CDN supply chain risk and supports offline-first architecture. The spec
PDF cannot be changed but this divergence is intentional.

---

## DEC-002: Pure-Go SQLite driver (no CGo)

**Date:** 2026-02-01 (CP1)
**Context:** Two main SQLite options in Go: `mattn/go-sqlite3` (CGo) and `modernc.org/sqlite` (pure Go).
**Decision:** Use `modernc.org/sqlite`, which registers as driver `"sqlite"` (not `"sqlite3"`).
**Rationale:** No CGo means no C compiler dependency, simpler cross-compilation, and easier
deployment. Pure Go binary with zero system library requirements.

---

## DEC-003: Flat package structure

**Date:** 2026-02-01 (CP1)
**Context:** Go projects can use sub-packages (`internal/handlers/`, `internal/db/`, etc.) or keep
everything in `package main`.
**Decision:** All `.go` files in `package main`, split by concern using filename suffixes
(`handlers_books.go`, `handlers_patrons.go`, etc.).
**Rationale:** The app is medium-sized. Sub-packages would add import indirection without meaningful
benefit at this scale.

---

## DEC-004: Session cookies over JWT

**Date:** 2026-03-01 (CP2)
**Context:** Two common approaches for web app auth: server-side sessions with cookies, or
stateless JWT tokens.
**Decision:** Server-side sessions stored in the `sessions` DB table, with `HttpOnly`,
`SameSite=Strict` cookies. Token generated with `crypto/rand`.
**Rationale:** Server-side sessions can be invalidated immediately (logout, compromise). JWTs
cannot be revoked without maintaining a blocklist, which negates the stateless benefit. For a
server-rendered app with no separate API clients, sessions are simpler and more secure.

---

## DEC-005: bcrypt for password hashing

**Date:** 2026-03-01 (CP2)
**Context:** Password hashing options include bcrypt, scrypt, argon2.
**Decision:** Use `golang.org/x/crypto/bcrypt` with default cost factor.
**Rationale:** bcrypt is well-understood, widely supported, and the cost factor provides natural
brute-force resistance. Good enough for this application; argon2 would be overkill.

---

## DEC-006: Public kiosk with optional login

**Date:** 2026-03-13 (CP2 design update)
**Context:** Original design had the kiosk as an authenticated page. Reconsidered whether public
browsing should require login.
**Decision:** `GET /kiosk` is fully public (no auth required). Patrons can optionally log in to
access favorites and holds. Checkout and return remain staff-only on the book detail page.
**Rationale:** A kiosk in a real library should let anyone walk up and browse. Requiring login
defeats the purpose. Patron features (favorites, holds) are value-adds that justify optional login.

---

## DEC-007: SSE for real-time availability (not WebSocket)

**Date:** 2026-03-01 (planned for CP6)
**Context:** Real-time availability updates need to push from server to browser.
**Decision:** Use Server-Sent Events (`GET /events`) instead of WebSocket.
**Rationale:** SSE is one-way (server to browser), which is exactly what this use case needs.
Simpler than WebSocket, works over HTTP/1.1, auto-reconnects, and requires no additional
dependencies.

---

## DEC-008: Open Library API via server-side proxy

**Date:** 2026-03-01 (planned for CP4)
**Context:** ISBN lookup needs to call the Open Library API to auto-fill book metadata.
**Decision:** Server-side proxy endpoint (`GET /api/openlibrary?isbn=...`) fetches data and
returns clean JSON to the client.
**Rationale:** Avoids CORS issues with direct browser requests. Keeps client JS simple. Allows
server-side ISBN format validation before making the outbound request.

---

## DEC-009: ZIP export/import with standard library

**Date:** 2026-03-01 (planned for CP7)
**Context:** Admin needs to export and import the full database and cover images.
**Decision:** Use Go's `archive/zip` standard library package.
**Rationale:** No third-party dependency needed. ZIP contains the SQLite file and cover images
from `static/images/covers/`.

---

## DEC-010: WAL mode for SQLite

**Date:** 2026-03-01 (CP2)
**Context:** SQLite default journal mode blocks readers during writes.
**Decision:** Enable WAL mode on startup (`PRAGMA journal_mode=WAL`).
**Rationale:** Required by the product specification. WAL allows concurrent reads during writes,
which matters when SSE connections are reading while staff actions are writing.

---

## DEC-011: Template loading pattern

**Date:** 2026-02-01 (CP1)
**Context:** Go templates can be loaded individually, as a global set, or as page-specific pairs.
**Decision:** `map[string]*template.Template` with one entry per page, each paired with
`layout.html`. The `renderTemplate()` helper executes the `"layout"` template which pulls in
the page's `"content"` block.
**Rationale:** Simple, explicit, and avoids the global template namespace collisions that come
from `template.ParseGlob()`. Each page knows exactly which templates it includes.
