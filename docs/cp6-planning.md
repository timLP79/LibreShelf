# CP6 Planning - Design Notes

Working document. Captures the state of design discussion to date. Not a finished plan - a place to pick up the conversation. Nothing in this file has been implemented yet.

**Context:**
- CP6 target close: 2026-04-27 (buffer day 2026-05-01)
- Today: 2026-04-19
- Starting point: `main` at tag v5 (CP5 complete, 93 tests passing)

---

## CP6 Scope (from CLAUDE.md, still authoritative)

- **#37** Pagination and filtering for catalog
- **#22** Loan system: checkout/return + kiosk browse + favorites (SSE and patron holds deferred post-submission)

**Added during design discussion:**
- Overdue notice reporting system with dashboard badge and printable notices
- Dashboard redesign (role-differentiated, new card set)
- Sidebar restructure (section headers, regrouping)
- Checkout/checkin portal (rapid-scan workflow for staff desk)

---

## Decisions Locked In

### Sequence
1. Pagination first (#37) - smaller, bounded, immediate demo win with 100-book catalog
2. Loan design session before any loan code
3. Loans backend
4. Loans UI (including checkout/checkin portal)
5. Kiosk layout
6. Favorites
7. Dashboard widget + overdue reports
8. CP6 close (tests, EC2 redeploy, merge)

### Loans schema
- `loans.due_date DATE NOT NULL`
- `loans.returned_at DATETIME` (nullable)
- `loans.fine_cents INTEGER NOT NULL DEFAULT 0` (future hook, no fine logic in CP6)
- No `status` column - the three states (active, returned, overdue) are expressible from `returned_at` and `due_date`
- Overdue is derived: `returned_at IS NULL AND due_date < DATE('now')`

### Overdue notice system
- Generated on-demand, not scheduled. No cron, no background jobs, no email.
- One notice per patron even if they have multiple overdue books.
- Browser print CSS (`@media print`, `@page`), not server-side PDF.
- New route: `/reports/overdue` - staff-facing table of overdue loans.
- New route: `/reports/overdue/patron/:id` - printable per-patron notice.

### Dashboard
- Role-differentiated at the template level. Same `/` route.
- Patrons land on the dashboard, not the kiosk. Kiosk is for public terminals.
- Cards propose-able below (still in flux).

### Bulk CSV import
- Pushed to CP7 alongside ZIP export/import (natural fit, #23 is already "bulk data management").
- CP7 scope widens to include CSV book import + CSV patron import.
- Keeps CP6 focused; CP7 gains a real headline feature instead of being pure polish.

### `/admin` route
- Retained, not retired. Earlier recommendation to delete was reversed after checking `docs/plan.md:408-409` - CP7 explicitly plans to put export/import + system stats on this route.
- Sidebar label changes from "Admin" (vague) to "Data Tools" (descriptive).
- URL stays `/admin`.

### Sidebar design principle
- Section headers describe the **domain of work**, not the access tier.
- Role gating is per-item (template conditionals), not per-section.
- This resolves the "Staff and Patrons are both people management but live in different sections" tension.

---

## Open Questions

Parked for the next design session. No decisions made yet.

1. **Sidebar structure - final grouping.** Current proposed shape (see "Sidebar Variants" below) is close but not committed. Specifically: does "Circulation" earn its own section, or do loan-related items live under Management?

2. **Dashboard card set - staff/admin view.** Four candidates:
   - Overdue (locked in)
   - Today's activity (N checkouts / M returns)
   - Active Loans count
   - Out of Stock count (Tim confirmed wants this for all users)

3. **Dashboard card set - patron view.**
   - My Active Loans (mini-list with due dates, not just count)
   - My Favorites (once favorites exist)
   - My Holds (placeholder; holds deferred post-submission)
   - Out of Stock (informational, click through to browse)

4. **Checkout portal variant.** See "Checkout Portal Design" below. Three variants (cart / rapid-scan / batch form). Recommendation: rapid-scan.

5. **Loan term default.** 14 days or 21 days? Single config constant for CP6; per-book override as a future enhancement.

6. **Patron address field.** Three options:
   - A. Add `patrons.address TEXT` (nullable), use on printed notice when present
   - B. Skip mailing address entirely; notice designed for hand-off when patron visits
   - C. Support optional address, design notice to look good with or without

7. **Receipt on checkout.** Print option on the portal after checkout, or skip for CP6?

8. **Checkout guardrails.**
   - Block checkout if patron has overdue items? (strict library policy)
   - Enforce a max loan count per patron? (if yes, what number?)
   - Both? Neither (trust staff)?

9. **Book-detail checkout.** Keep the per-book checkout form on the book detail page as a shortcut, or remove it now that the portal exists?

10. **Bulk-print overdue notices.** "Print All" button that opens all notices paginated, or only per-patron printing?

---

## Sidebar Variants (pick one, refine later)

### Variant 1 - Original proposal (section headers as access tiers)

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

### Variant 2 - Reorganized (section headers as domains)

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

### Variant 3 - Add Circulation section (post checkout-portal discussion)

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

Four sections. "Circulation" earns a section because checkout/checkin/loans/reports are the highest-frequency staff work and they form a coherent domain. Loans and Reports move out of Management into Circulation. Management shrinks to people-only.

**Currently leaning:** Variant 3. Still subject to more thought.

---

## Dashboard Card Set Proposals

### Staff / admin view

Design principle: every card answers one of:
1. "Is anything urgent?" (alert state)
2. "What's happening right now?" (ambient pulse)
3. "Where do I go to act?" (navigation hint)

If a card doesn't answer one of those, it's decoration.

**Proposed four cards:**

| Card | Signal | Click target |
|---|---|---|
| Overdue | Red danger badge with count when > 0; muted "All loans current" when 0 | `/reports/overdue` |
| Today's Activity | "N checkouts, M returns today" (single card, two numbers) | `/loans?since=today` or similar |
| Active Loans | Current count of non-returned loans | `/loans` (loan management) |
| Out of Stock | Count of titles where `quantity_available = 0` | Filtered catalog view |

### Patron view

| Card | Signal | Click target |
|---|---|---|
| My Active Loans | Mini-list of titles + due dates; "Due in 2 days" highlighted when close | `/my-loans` |
| My Favorites | Mini-list or count | `/my-favorites` |
| My Holds | Placeholder until holds ship (post-submission) | `/my-holds` when built |
| Out of Stock | Count with click through to list; ties into holds feature later | Filtered catalog |

### Cards explicitly cut

- Books count (static, no signal)
- Patrons count (low signal, rarely changes)
- Staff count (admin reaches staff via nav)

---

## Checkout Portal Design

### Why a portal exists

Per-book checkout from the catalog is wrong workflow for a library desk. A patron comes to the desk with a stack, not one book at a time. The portal is a dedicated page optimized for that flow.

### Three variants

**Variant A - Cart model.**
- Staff picks patron
- Adds books one at a time to a cart
- Reviews cart
- Submits once
- Pros: review step before commit, safer
- Cons: slower at the desk, extra click per item

**Variant B - Rapid-scan (recommended).**
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

### Returns portal

- No patron selection needed - loan record identifies who borrowed the book
- Same rapid-scan UI: scan ISBN, system finds the active loan, marks returned, shows confirmation
- Edge case: ISBN not associated with any active loan -> clear error message, do not block further entries

### Routing

Two dedicated pages:
- `/checkout` - patron + rapid-scan ISBN entry
- `/checkin` - rapid-scan ISBN entry

Separate because the mental mode and form shape differ meaningfully. Combined `/circulation` with tabs is also possible but slightly more friction.

### Backend implications

- Loan creation handler accepts `patron_id + []isbn`, not just one book
- One transaction per submitted book (or one transaction per session? decide later)
- Error reporting: if book 3 of 5 fails (out of stock), books 1-2 succeeded are committed; book 3+ rejected with an inline message. Staff continues.

### Book-detail checkout

- Open question #9: keep as a secondary shortcut or remove?
- Argument for keep: a patron browsing from the kiosk might want to initiate checkout from the book detail page, then staff completes it via the portal. But that's a hold request, not a checkout.
- Argument for remove: simpler UX, one way to check out books
- Leaning: remove the per-book checkout form on the book detail page. Keep the page read-only for logged-in staff.

---

## Overdue Notice System Summary

### Trigger
On-demand. Staff navigates to `/reports/overdue`, reviews the table, clicks per-patron "Print Notice" to open the printable notice.

### Row granularity - hybrid
- Table rows: per-loan (Jane Doe has 3 overdue books = 3 rows). Shows book title, due date, days overdue per row.
- Notice (on row click): per-patron (one notice listing all her overdue books)
- Badge count: total overdue loans (not total overdue patrons). When Jane has 3 overdue books, the dashboard badge says "3".

### Notice content
- Library header (name, date of notice)
- Patron name + contact info (email, phone; address if we add that field)
- List of overdue books: title, author, due date, days overdue
- Standard text ("Please return the following items...")
- Library contact footer

### Print CSS
- `@media print` hides nav/sidebar
- `@page { margin: 1in; }`
- `page-break-after: always` between patrons on a bulk print
- Serif font for formal look

### Queries

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

---

## Session Estimate

**Current projection: 8-9 sessions.**

| # | Session | Scope | Est. hrs |
|---|---|---|---|
| 1 | Pagination (#37) | Server-side LIMIT/OFFSET, search/genre filter, page nav, template + JS, tests | 2-3 |
| 2 | Design session | Loan state machine, schema decisions, dashboard + sidebar finalization, portal variant pick, DEC writeups. No code. | 1.5-2 |
| 3 | Loans backend | DB methods (checkout, return, history, active loans), transactional writes, sentinel errors (ErrNoCopiesAvailable, ErrLoanAlreadyReturned), handlers | 3-4 |
| 4 | Checkout/checkin portal | `/checkout` and `/checkin` pages, rapid-scan flow, session state, backend bulk endpoint, tests | 3-4 |
| 5 | Kiosk page | `/kiosk` route, anonymous browse, optional patron login gate | 2-3 |
| 6 | Favorites | `patron_favorites` table, toggle handler, heart/star UI on kiosk, "My Favorites" list | 2-3 |
| 7 | Dashboard + sidebar | Card backfills (real counts), Overdue card, role branches, sidebar restructure, nav section headers | 2-3 |
| 8 | Reports + notices | `/reports/overdue` table, per-patron notice detail view, print CSS, tests | 2-3 |
| 9 | CP6 close | Integration smoke, test pass at role boundaries, EC2 redeploy with clean DB, PR + merge | 1-2 |

**Total: ~19-27 hours across 8-9 sessions.**

Sessions could compress by combining 7 and 8 (dashboard + reports landed together since both touch overdue queries). That brings it to 7-8 sessions.

---

## Proposed DEC Entries

To write in session 2 (design), not now.

- **DEC-024** Loan state model via `due_date + returned_at`, no status column. Implicit states (active, returned, overdue).
- **DEC-025** Overdue notices generated on-demand, per-patron grouped, browser-print CSS. No scheduled job, no server PDF.
- **DEC-026** Dashboard role-differentiated. Same `/` route, template branches on role. Patrons land here, not kiosk.
- **DEC-027** Sidebar sections describe domain of work, not access tier. Role gating is per-item.
- **DEC-028** `/admin` route retained; sidebar label renamed to "Data Tools". Route is the CP7 home for export/import + settings.
- **DEC-029** `fine_cents` column reserved on loans table. No fine feature in CP6; shape decided in a future CP when requirements are real.
- **DEC-030** Checkout portal is the primary staff workflow. Per-book checkout on the book detail page either removed or kept as a shortcut (open question).

---

## Reasoning Log

Notes to self on decisions that evolved during discussion.

### Earlier recommendation: retire `/admin` route
**Reversed.** Recommended retirement based on current template being empty. Missed that `docs/plan.md:408-409` explicitly plans CP7 content for that route. Lesson: check the roadmap before recommending deletion, not just current content.

### Earlier recommendation: sidebar groups by access tier
**Reversed.** Section headers "Management (staff+admin)" and "Administration (admin-only)" conflated domain with role. Fixed by making section headers describe domains and gating individual items. Better matches how the conceptual space actually splits.

### Bulk CSV import scope
Considered in CP6 briefly. Moved to CP7 because:
- CP6 is already large with loans + kiosk + favorites + overdue + portal + dashboard redesign
- CP7's #23 is already the "bulk data management" checkpoint; CSV is a natural sibling
- CP7 currently has room that CP6 does not

### Dashboard card pruning
Cut Books (static, no signal) and Patrons (rarely changes, low signal) count cards. Out-of-stock replaces them as an actionable card. Staff count was never proposed.

---

## Next Steps

When picking this up:

1. Decide sidebar variant (1, 2, or 3) and lock in section names.
2. Decide checkout portal variant (A, B, or C).
3. Decide patron address question (A, B, or C) - drives patron schema change.
4. Decide loan term number (14 or 21).
5. Decide guardrails question (overdue-block? max-loans?).
6. Decide receipt question (yes/no/defer).
7. Decide book-detail checkout question (keep/remove).
8. Write DEC entries 024-030 in DECISIONS.md.
9. Begin session 1 (pagination).

Nothing has been committed to code yet. All changes are reversible as pure doc discussion.
