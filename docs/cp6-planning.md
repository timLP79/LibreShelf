# CP6 Planning

**Context:**
- CP6 target close: 2026-04-27 (buffer day 2026-05-01)
- Today: 2026-04-19
- Starting point: `main` at tag v5 (CP5 complete, 93 tests passing)

This plan uses the disciplined scope from the v2 reality-check: CP6 ships the foundation that makes the software function. Workflow polish (rapid-scan portal, sidebar restructure, dashboard redesign, printed overdue notices, patron-facing mini-lists) moves to the post-submission backlog. Those designs are preserved at the bottom of this doc so they can be picked up later without re-designing.

---

## CP6 Scope

### 1. Loan schema + DB methods (transactional, DEC-022 pattern)

- `loans.due_date DATE NOT NULL`
- `loans.returned_at DATETIME` (nullable)
- `loans.fine_cents INTEGER NOT NULL DEFAULT 0` (future hook, no fine logic)
- No `status` column: the three states (active, returned, overdue) are expressible from `returned_at` and `due_date`.
- Overdue derived: `returned_at IS NULL AND due_date < DATE('now')`.
- DB methods: `CheckoutBook`, `ReturnBook`, `GetActiveLoans`, `GetOverdueLoans`, `GetLoanHistoryByBook`, `GetLoanHistoryByPatron`.
- Both writes are transactional (loan row + `quantity_available` adjustment in one tx).

### 2. Checkout/return handlers wired to existing book-detail scaffold

- `HandleCheckout` and `HandleReturn` in a new `handlers_loans.go`.
- Staff-only route group.
- Inputs: `book_id`, `patron_id`, form submitted from the book-detail page's existing (currently disabled) scaffold.
- Sentinel errors: `ErrNoCopiesAvailable`, `ErrLoanAlreadyReturned`, `ErrPatronNotFound`.
- Flash-cookie messaging on success and failure (new codes added to `flashMessages` in flash.go).

### 3. `/loans` page with active/overdue filter

- Single template, no print CSS, no per-patron grouping, no notice rendering.
- Table columns: book title, patron name, due date, days overdue (derived), return action.
- Filter (query param `?filter=active|overdue`, default `active`) toggles between the two views.
- Each row has a "Return" button that POSTs to the return handler.
- Sorted by due date ascending (oldest/most-overdue first).

### 4. Pagination (#37)

- Server-side `LIMIT` + `OFFSET` on `/` catalog.
- Query params: `?page=N&q=<search>&genre=<genre>`.
- Page nav component (previous / current / next / last) at the bottom of the catalog.
- Default page size: 24 (fits 6-wide x 4-rows on desktop, 4-wide x 6-rows acceptable).
- Remove client-side filtering JS from `app.js` for the catalog; search and genre filtering now round-trip to the server.

### 5. Dashboard: wire placeholder cards to real counts

- No redesign. Existing `templates/index.html` keeps its current three-card layout.
- Replace the three "-" placeholders with real `SELECT COUNT(...)` queries:
  - Books (all users)
  - Patrons (staff + admin only)
  - Active Loans (all users)
- Patron/staff role gating on the Patrons card stays as-is.
- No Overdue card, no Today's Activity card, no Out of Stock card in CP6. Those are deferred.

### 6. Kiosk public browse

- `/kiosk` route, publicly accessible (no auth required).
- Reuses catalog grid UI, minus staff controls (no Edit/Delete buttons).
- No patron login gate in CP6; anonymous browse only.

### 7. Favorites (if time permits)

- `patron_favorites` table, small handler to toggle, heart icon on kiosk book cards.
- Only if sessions 1-6 ship ahead of 4/27. Otherwise defers to CP7 or post-submission.

---

## Decisions to Record

Three DEC entries to write in session 2 (design session). Do not write DECs for deferred work.

- **DEC-024** Loan state expressed via `due_date + returned_at` columns. No `status` column; the three states (active, returned, overdue) are derived. Rationale: schema simplicity, no denormalization that could drift.
- **DEC-025** Loan list view is a single `/loans` page with an `active | overdue` query-param filter. No per-patron grouping, no printable notice rendering in CP6. Rationale: the CP6 scope needs a way to see and act on loans; formatted notices are volume-optimization workflow polish deferred post-submission.
- **DEC-026** `loans.fine_cents INTEGER NOT NULL DEFAULT 0` reserved on schema. No fine feature in CP6. Shape of fine management (column extension vs separate `fines` table) deferred to a future CP when requirements are real.

---

## Session Estimate

**6-7 sessions. Fits inside the 8-day window to 4/27 with buffer.**

| # | Session | Scope | Est. hrs |
|---|---|---|---|
| 1 | Pagination (#37) | Server-side LIMIT/OFFSET, search/genre filter, page nav, template + JS, tests | 2-3 |
| 2 | Loan design | Schema confirmation, DEC-024/025/026, handler shapes, error sentinels. No code. | 1.5-2 |
| 3 | Loans backend | DB methods (transactional), handlers (checkout, return), tests | 3-4 |
| 4 | Loans UI + dashboard counts | Wire book-detail scaffold to handlers, build `/loans` page with filter, replace dashboard placeholders with real counts | 3-4 |
| 5 | Kiosk public browse | `/kiosk` route, anonymous browse, reused catalog grid minus staff controls | 2-3 |
| 6 | Favorites (if time) | `patron_favorites`, toggle handler, heart UI | 2-3 |
| 7 | CP6 close | Integration smoke, role-boundary tests, EC2 redeploy with clean DB, PR + merge | 1-2 |

**Total: ~14-21 hours. Compresses to 5 sessions if favorites defers.**

---

## CP7 Scope (unchanged, confirming here for completeness)

- **#23** Bulk data management: ZIP export/import (admin) + CSV book/patron import (staff, absorbed from CP6 design discussion)
- **#24** Testing, polish, deploy: `SecurityHeaders` middleware, `SetTrustedProxies`, `go mod verify`, `govulncheck`, final EC2 redeploy with a clean DB to pick up new seed passwords

---

## Deferred Post-Submission Design Notes

The following sections preserve design work from the initial CP6 discussion that was trimmed by the v2 reality-check. None of this is in CP6 scope. Picked up post-submission when priorities allow.

### Checkout/Checkin Portal (rapid-scan workflow)

**Why a portal exists:** Per-book checkout from the catalog is wrong workflow for a library desk at volume. A patron comes to the desk with a stack, not one book at a time. For a library with 10-50 transactions per day the per-book flow on the book-detail scaffold is workable; the portal earns its place at higher volume.

**Three design variants considered:**

**Variant A - Cart model.**
- Staff picks patron
- Adds books one at a time to a cart
- Reviews cart
- Submits once
- Pros: review step before commit, safer against typos
- Cons: slower at the desk, extra click per item

**Variant B - Rapid-scan (recommended if built).**
- Staff picks patron once at the top of the page
- A focused input field accepts ISBN entry
- Pressing Enter on an ISBN fires an immediate checkout
- Running list below shows this session's checkouts
- Session ends when staff closes the page or picks a new patron
- Pros: highest throughput, scanner-compatible (scanners are just keyboards + Enter), errors surface inline
- Cons: less forgiving of typos; no review step

**Variant C - Batch form.**
- Select patron
- Textarea or dynamic field list for ISBNs
- Submit the whole form at once
- Pros: familiar form UX
- Cons: bad for a physical desk with a stack of books; bad for scanner use

**Returns portal:** No patron selection needed (loan record identifies who borrowed the book). Same rapid-scan UI.

**Routing:** Two dedicated pages, `/checkout` and `/checkin`. Separate because mental mode and form shape differ meaningfully. Combined `/circulation` with tabs also possible.

**Backend implications:** Loan creation handler accepts `patron_id + []isbn`, not just one book. Error reporting: if book 3 of 5 fails (out of stock), books 1-2 succeeded are committed; book 3 rejected with inline message, staff continues with 4-5.

---

### Sidebar Restructure

The current sidebar has "Admin" and "Staff" as separate top-level items, which conflates two problems: "Admin" is a vague label, and Staff management is admin-only but lives as a peer to Admin rather than inside it.

**Design principle landed on:** Section headers describe the **domain of work**, not the access tier. Role gating is per-item (template conditional), not per-section.

**Three sidebar variants considered:**

**Variant 1 - Original (sections as access tiers).**

```
- Navigation -
Dashboard
Catalog
Kiosk

- Management -       (staff + admin)
Patrons
Active Loans
Reports

- Administration -   (admin only)
Staff
Data Tools
```

Problem: Staff and Patrons are both "people management" but sit in different sections purely because of access control.

**Variant 2 - Reorganized (sections as domains).**

```
- Navigation -
Dashboard
Catalog
Kiosk

- Management -       (role-gated per item)
Patrons          (staff + admin)
Staff            (admin only)
Active Loans     (staff + admin)
Reports          (staff + admin)

- Administration -   (admin only)
Data Tools
```

Fixes the Variant 1 problem. Section labels describe the work, not the access tier.

**Variant 3 - Add Circulation section (emerged after portal discussion).**

```
- Navigation -
Dashboard
Catalog
Kiosk

- Circulation -      (staff + admin)
Check Out        -> /checkout
Check In         -> /checkin
Active Loans     -> /loans
Reports          -> /reports/overdue

- Management -
Patrons          (staff + admin)
Staff            (admin only)

- Administration -   (admin only)
Data Tools
```

Four sections. "Circulation" earns a section because checkout/checkin/loans/reports are the highest-frequency staff work and they form a coherent domain.

**Leaning if this gets built:** Variant 3. Not committed.

**`/admin` route disposition:** Retained, not retired. Earlier recommendation to delete was reversed after checking `docs/plan.md:408-409` - CP7 explicitly plans export/import + system stats content for this route. Sidebar label changes from "Admin" to "Data Tools" to describe the page's purpose.

---

### Dashboard Redesign (role-differentiated, new card set)

**Design principle:** every card answers one of:
1. "Is anything urgent?" (alert state)
2. "What's happening right now?" (ambient pulse)
3. "Where do I go to act?" (navigation hint)

Cards that don't answer one of those are decoration.

**Staff / admin view (proposed four cards):**

| Card | Signal | Click target |
|---|---|---|
| Overdue | Red danger badge with count when > 0; muted "All loans current" when 0 | `/reports/overdue` |
| Today's Activity | "N checkouts, M returns today" (single card, two numbers) | loan activity log |
| Active Loans | Current count of non-returned loans | `/loans` |
| Out of Stock | Count of titles where `quantity_available = 0` | filtered catalog |

**Patron view:**

| Card | Signal | Click target |
|---|---|---|
| My Active Loans | Mini-list of titles + due dates; "Due in 2 days" highlighted when close | `/my-loans` |
| My Favorites | Mini-list or count | `/my-favorites` |
| My Holds | Placeholder until holds ship | `/my-holds` when built |
| Out of Stock | Count with click through; ties to holds feature | filtered catalog |

**Cards explicitly cut:** Books count (static, no signal), Patrons count (low signal, rarely changes), Staff count (admin reaches staff via nav).

---

### Overdue Notice Print System

**Trigger:** On-demand. Staff navigates to `/reports/overdue`, reviews the table, clicks per-patron "Print Notice".

**Row granularity - hybrid:**
- Table rows: per-loan. Jane Doe has 3 overdue books = 3 rows. Shows book title, due date, days overdue.
- Notice (on row click): per-patron. One notice listing all her overdue books.
- Badge count: total overdue loans (not patrons). Jane has 3 overdue books = badge shows "3".

**Notice content:**
- Library header (name, date of notice)
- Patron name + contact info (email, phone; address if that field is ever added)
- List of overdue books: title, author, due date, days overdue
- Standard text ("Please return the following items...")
- Library contact footer

**Print CSS:**
- `@media print` hides nav/sidebar
- `@page { margin: 1in; }`
- `page-break-after: always` between patrons on bulk print
- Serif font for formal look

**Queries:**

```sql
-- Badge count
SELECT COUNT(*) FROM loans
WHERE returned_at IS NULL AND due_date < DATE('now');

-- Reports table rows (per-loan granularity)
SELECT l.id, l.due_date, p.id AS patron_id, p.name,
       b.title, b.isbn,
       CAST(julianday('now') - julianday(l.due_date) AS INTEGER) AS days_overdue
FROM loans l
JOIN patrons p ON l.patron_id = p.id
JOIN books b ON l.book_id = b.id
WHERE l.returned_at IS NULL AND l.due_date < DATE('now')
ORDER BY l.due_date ASC;
```

**Scope note:** CP6 ships the "know what's overdue" part (item 3 in CP6 Scope, via the `/loans?filter=overdue` filter). The printable per-patron notice with CSS styling is the deferred polish.

---

### Patron Address Field (schema question)

Only relevant if the printed notice system is built. Three options:

- **A.** Add `patrons.address TEXT` (nullable), use on printed notice when present
- **B.** Skip mailing address entirely; notice designed for hand-off when patron visits
- **C.** Support optional address, design notice to look good with or without

No decision needed until the notice system is built.

---

### Other Deferred Items (mentioned during discussion)

- **Loan term default** (14 vs 21 days): one config constant; per-book override a future enhancement.
- **Receipt on checkout**: print option on the portal after checkout. Deferred with the portal.
- **Checkout guardrails**: block checkout if patron has overdue items? enforce max loan count? Not decided; trust staff for V1.
- **Book-detail checkout vs portal**: if the portal is ever built, decide whether to keep per-book checkout as a shortcut or remove.
- **Bulk-print overdue notices**: "Print All" that opens all notices paginated, vs per-patron only.
- **Holds system (#22 partial)**: deferred with SSE per CLAUDE.md.
- **Favorites UI polish**: the CP6 version is functional; visual polish (animation, sort order, empty state) can improve later.

---

## Reasoning Log

Notes on decisions that evolved during the CP6 design discussion, preserved so future-me doesn't repeat the same thinking.

**Earlier recommendation: retire `/admin` route.**
Reversed. Recommended retirement based on current template being empty. Missed that `docs/plan.md:408-409` explicitly plans CP7 content for that route. Lesson: check the roadmap before recommending deletion, not just present-tense content.

**Earlier recommendation: sidebar groups by access tier.**
Reversed. Section headers "Management (staff+admin)" and "Administration (admin-only)" conflated domain with role. Fixed by making section headers describe domains and gating individual items.

**Earlier recommendation: checkout portal is CP6-core.**
Reversed by v2. Portal is volume workflow polish, not foundation. Small-library per-book flow works. Portal moves to post-submission.

**Earlier recommendation: dashboard redesign + new card set in CP6.**
Reversed by v2. Wiring existing placeholders to real counts is CP6. Redesign is polish.

**Earlier recommendation: overdue notice print system in CP6.**
Reversed by v2. The "know what's overdue" need is satisfied by a filter on `/loans`. The printable formatted notice with `@media print` CSS is polish, deferred.

**Bulk CSV import scope:**
Considered in CP6 briefly, moved to CP7 because CP6 is already full and CP7's #23 is already "bulk data management" territory. CSV is a natural sibling of ZIP.

**Discipline lens that should have been applied throughout:**
"Does CP6 need this to function?" Not "would this be nice at volume?" not "would a good product manager want this?" The foundation lens is the right lens for a checkpoint with a hard deadline. Polish comes after the foundation ships.
