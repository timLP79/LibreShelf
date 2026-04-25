package main

import (
	"database/sql"
	"testing"
	"time"
)

// mustCreateBook seeds a book with the given total/available quantity and
// a single "Test Author" credit. Returns the book id.
func mustCreateBook(t *testing.T, dm *DatabaseManager, title string, quantity int) int {
	t.Helper()
	book := &Book{Title: title, QuantityTotal: quantity, QuantityAvailable: quantity}
	id, err := dm.CreateBook(book, []string{"Test Author"})
	if err != nil {
		t.Fatalf("CreateBook(%q): %v", title, err)
	}
	return id
}

// mustCreatePatron seeds a patron (and its linked user row via the
// transactional CreatePatron path). Returns the patron id.
func mustCreatePatron(t *testing.T, dm *DatabaseManager, name string) int {
	t.Helper()
	id, _, err := dm.CreatePatron(name, "", "", "fake-hash")
	if err != nil {
		t.Fatalf("CreatePatron(%q): %v", name, err)
	}
	return id
}

// mustInsertLoan bypasses CheckoutBook to seed loans directly. Needed for
// tests that exercise guard conditions (overdue, at-limit) and filters
// (GetActiveLoans, GetOverdueLoans). Pass empty string for returnedAt to
// leave the loan active.
func mustInsertLoan(t *testing.T, dm *DatabaseManager, bookID, patronID int, dueDate, returnedAt string) int {
	t.Helper()
	var (
		res sql.Result
		err error
	)
	if returnedAt == "" {
		res, err = dm.db.Exec(
			`INSERT INTO loans (book_id, patron_id, due_date) VALUES (?, ?, ?)`,
			bookID, patronID, dueDate)
	} else {
		res, err = dm.db.Exec(
			`INSERT INTO loans (book_id, patron_id, due_date, returned_at) VALUES (?, ?, ?, ?)`,
			bookID, patronID, dueDate, returnedAt)
	}
	if err != nil {
		t.Fatalf("insert loan: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return int(id)
}

// TestCheckoutBookHappyPath pins the baseline success path: a loan row is
// created and quantity_available is decremented. Both changes must land
// together since they share a transaction.
func TestCheckoutBookHappyPath(t *testing.T) {
	dm := setupTestDB(t)
	bookID := mustCreateBook(t, dm, "Test Book", 2)
	patronID := mustCreatePatron(t, dm, "Jane Doe")
	dueDate := time.Now().AddDate(0, 0, DefaultLoanTermDays)

	if err := dm.CheckoutBook(bookID, patronID, dueDate); err != nil {
		t.Fatalf("CheckoutBook: %v", err)
	}

	var count int
	if err := dm.db.QueryRow(
		`SELECT COUNT(*) FROM loans
		 WHERE book_id = ? AND patron_id = ? AND returned_at IS NULL`,
		bookID, patronID).Scan(&count); err != nil {
		t.Fatalf("count loans: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 active loan, got %d", count)
	}

	var available int
	if err := dm.db.QueryRow(`SELECT quantity_available FROM books WHERE id = ?`, bookID).Scan(&available); err != nil {
		t.Fatalf("query quantity: %v", err)
	}
	if available != 1 {
		t.Errorf("expected quantity_available=1, got %d", available)
	}
}

// TestCheckoutBookNoCopiesAvailable pins the "book is out of stock" guard.
// Without this test, a race between two staff checking out the last copy
// could produce a negative quantity_available.
func TestCheckoutBookNoCopiesAvailable(t *testing.T) {
	dm := setupTestDB(t)
	bookID := mustCreateBook(t, dm, "Zero Copies", 0)
	patronID := mustCreatePatron(t, dm, "Alice")
	dueDate := time.Now().AddDate(0, 0, DefaultLoanTermDays)

	err := dm.CheckoutBook(bookID, patronID, dueDate)
	if err != ErrNoCopiesAvailable {
		t.Errorf("expected ErrNoCopiesAvailable, got %v", err)
	}
}

// TestCheckoutBookBlockedByOverdue pins the overdue guard. A patron with
// even one overdue book cannot check out anything new, regardless of
// book availability.
func TestCheckoutBookBlockedByOverdue(t *testing.T) {
	dm := setupTestDB(t)
	bookA := mustCreateBook(t, dm, "Overdue Book", 1)
	bookB := mustCreateBook(t, dm, "Wanted Book", 5)
	patronID := mustCreatePatron(t, dm, "Overdue Olivia")

	yesterday := time.Now().AddDate(0, 0, -1).UTC().Format("2006-01-02")
	mustInsertLoan(t, dm, bookA, patronID, yesterday, "")

	dueDate := time.Now().AddDate(0, 0, DefaultLoanTermDays)
	err := dm.CheckoutBook(bookB, patronID, dueDate)
	if err != ErrPatronHasOverdue {
		t.Errorf("expected ErrPatronHasOverdue, got %v", err)
	}
}

// TestCheckoutBookAtLoanLimit pins the max-active-loans guard at the
// exact threshold. MaxActiveLoansPerPatron active loans must cause the
// next checkout to fail with ErrPatronAtLoanLimit.
func TestCheckoutBookAtLoanLimit(t *testing.T) {
	dm := setupTestDB(t)
	patronID := mustCreatePatron(t, dm, "Maxed Max")
	nextWeek := time.Now().AddDate(0, 0, 7).UTC().Format("2006-01-02")

	for i := range MaxActiveLoansPerPatron {
		bookID := mustCreateBook(t, dm, "Loan Filler Book "+string(rune('A'+i)), 1)
		mustInsertLoan(t, dm, bookID, patronID, nextWeek, "")
	}

	oneMore := mustCreateBook(t, dm, "The Straw", 1)
	err := dm.CheckoutBook(oneMore, patronID, time.Now().AddDate(0, 0, DefaultLoanTermDays))
	if err != ErrPatronAtLoanLimit {
		t.Errorf("expected ErrPatronAtLoanLimit, got %v", err)
	}
}

// TestCheckoutBookAtLimitBoundary pins the other side of the limit: a
// patron with MaxActiveLoansPerPatron - 1 active loans CAN still check
// out one more. Regression guard against an off-by-one in the comparator.
func TestCheckoutBookAtLimitBoundary(t *testing.T) {
	dm := setupTestDB(t)
	patronID := mustCreatePatron(t, dm, "One Under Max")
	nextWeek := time.Now().AddDate(0, 0, 7).UTC().Format("2006-01-02")

	for i := range MaxActiveLoansPerPatron - 1 {
		bookID := mustCreateBook(t, dm, "Loan Filler Book "+string(rune('A'+i)), 1)
		mustInsertLoan(t, dm, bookID, patronID, nextWeek, "")
	}

	final := mustCreateBook(t, dm, "The Last One", 1)
	if err := dm.CheckoutBook(final, patronID, time.Now().AddDate(0, 0, DefaultLoanTermDays)); err != nil {
		t.Errorf("expected success at boundary, got %v", err)
	}
}

// TestReturnBookHappyPath pins the return round-trip: returned_at is
// stamped and quantity_available is restored in one transaction.
func TestReturnBookHappyPath(t *testing.T) {
	dm := setupTestDB(t)
	bookID := mustCreateBook(t, dm, "Return Target", 1)
	patronID := mustCreatePatron(t, dm, "Returnee")

	if err := dm.CheckoutBook(bookID, patronID, time.Now().AddDate(0, 0, DefaultLoanTermDays)); err != nil {
		t.Fatalf("CheckoutBook: %v", err)
	}

	var loanID int
	if err := dm.db.QueryRow(
		`SELECT id FROM loans WHERE book_id = ? AND patron_id = ?`,
		bookID, patronID).Scan(&loanID); err != nil {
		t.Fatalf("query loan id: %v", err)
	}

	if err := dm.ReturnBook(loanID); err != nil {
		t.Fatalf("ReturnBook: %v", err)
	}

	var returnedAt sql.NullString
	if err := dm.db.QueryRow(`SELECT returned_at FROM loans WHERE id = ?`, loanID).Scan(&returnedAt); err != nil {
		t.Fatalf("query returned_at: %v", err)
	}
	if !returnedAt.Valid {
		t.Errorf("expected returned_at set, got NULL")
	}

	var available int
	if err := dm.db.QueryRow(`SELECT quantity_available FROM books WHERE id = ?`, bookID).Scan(&available); err != nil {
		t.Fatalf("query quantity: %v", err)
	}
	if available != 1 {
		t.Errorf("expected quantity_available restored to 1, got %d", available)
	}
}

// TestReturnBookAlreadyReturned pins the idempotency guard. Returning a
// second time must not increment quantity_available again; without this
// guard, a browser refresh after return would over-count copies.
func TestReturnBookAlreadyReturned(t *testing.T) {
	dm := setupTestDB(t)
	bookID := mustCreateBook(t, dm, "Already Returned", 1)
	patronID := mustCreatePatron(t, dm, "Quick Returner")

	if err := dm.CheckoutBook(bookID, patronID, time.Now().AddDate(0, 0, DefaultLoanTermDays)); err != nil {
		t.Fatalf("CheckoutBook: %v", err)
	}
	var loanID int
	if err := dm.db.QueryRow(`SELECT id FROM loans WHERE book_id = ?`, bookID).Scan(&loanID); err != nil {
		t.Fatalf("query loan id: %v", err)
	}
	if err := dm.ReturnBook(loanID); err != nil {
		t.Fatalf("first ReturnBook: %v", err)
	}

	err := dm.ReturnBook(loanID)
	if err != ErrLoanAlreadyReturned {
		t.Errorf("expected ErrLoanAlreadyReturned on second return, got %v", err)
	}

	var available int
	if err := dm.db.QueryRow(`SELECT quantity_available FROM books WHERE id = ?`, bookID).Scan(&available); err != nil {
		t.Fatalf("query quantity: %v", err)
	}
	if available != 1 {
		t.Errorf("expected quantity_available=1 after double-return attempt, got %d", available)
	}
}

// TestReturnBookNotFound pins the "loan does not exist" path. Must
// surface sql.ErrNoRows so the handler can 404 instead of 500.
func TestReturnBookNotFound(t *testing.T) {
	dm := setupTestDB(t)

	err := dm.ReturnBook(99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// TestGetActiveLoansFiltersReturnedAndOverdue pins the active filter:
// only loans where returned_at IS NULL and due_date >= today appear.
// Returned loans and overdue loans must be excluded.
func TestGetActiveLoansFiltersReturnedAndOverdue(t *testing.T) {
	dm := setupTestDB(t)
	bookA := mustCreateBook(t, dm, "Active Book", 1)
	bookB := mustCreateBook(t, dm, "Returned Book", 1)
	bookC := mustCreateBook(t, dm, "Overdue Book", 1)
	patronID := mustCreatePatron(t, dm, "Pat")

	nextWeek := time.Now().AddDate(0, 0, 7).UTC().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).UTC().Format("2006-01-02")

	mustInsertLoan(t, dm, bookA, patronID, nextWeek, "")                             // active
	mustInsertLoan(t, dm, bookB, patronID, nextWeek, "2026-04-01 12:00:00")          // returned
	mustInsertLoan(t, dm, bookC, patronID, yesterday, "")                            // overdue

	loans, err := dm.GetActiveLoans()
	if err != nil {
		t.Fatalf("GetActiveLoans: %v", err)
	}
	if len(loans) != 1 {
		t.Fatalf("expected 1 active loan, got %d", len(loans))
	}
	if loans[0].BookID != bookA {
		t.Errorf("expected book %d, got %d", bookA, loans[0].BookID)
	}
}

// TestGetOverdueLoansOnlyPastDue pins two things: only loans with
// due_date < today are returned, and DaysOverdue is computed correctly.
func TestGetOverdueLoansOnlyPastDue(t *testing.T) {
	dm := setupTestDB(t)
	bookA := mustCreateBook(t, dm, "3 Days Overdue", 1)
	bookB := mustCreateBook(t, dm, "Active", 1)
	patronID := mustCreatePatron(t, dm, "Pat")

	threeDaysAgo := time.Now().AddDate(0, 0, -3).UTC().Format("2006-01-02")
	nextWeek := time.Now().AddDate(0, 0, 7).UTC().Format("2006-01-02")

	mustInsertLoan(t, dm, bookA, patronID, threeDaysAgo, "")
	mustInsertLoan(t, dm, bookB, patronID, nextWeek, "")

	loans, err := dm.GetOverdueLoans()
	if err != nil {
		t.Fatalf("GetOverdueLoans: %v", err)
	}
	if len(loans) != 1 {
		t.Fatalf("expected 1 overdue loan, got %d", len(loans))
	}
	if loans[0].DaysOverdue != 3 {
		t.Errorf("expected DaysOverdue=3, got %d", loans[0].DaysOverdue)
	}
}

// TestGetPatronActiveLoansScopedToPatron pins that the patron-scoped
// filter returns only the given patron's active loans, not everyone's.
func TestGetPatronActiveLoansScopedToPatron(t *testing.T) {
	dm := setupTestDB(t)
	book := mustCreateBook(t, dm, "Shared Book Title Space", 3)
	pat1 := mustCreatePatron(t, dm, "Patron One")
	pat2 := mustCreatePatron(t, dm, "Patron Two")
	nextWeek := time.Now().AddDate(0, 0, 7).UTC().Format("2006-01-02")

	mustInsertLoan(t, dm, book, pat1, nextWeek, "")
	mustInsertLoan(t, dm, book, pat1, nextWeek, "")
	mustInsertLoan(t, dm, book, pat2, nextWeek, "")

	loans, err := dm.GetPatronActiveLoans(pat1)
	if err != nil {
		t.Fatalf("GetPatronActiveLoans: %v", err)
	}
	if len(loans) != 2 {
		t.Errorf("expected 2 loans for patron 1, got %d", len(loans))
	}
	for _, l := range loans {
		if l.PatronID != pat1 {
			t.Errorf("unexpected patron id %d in scoped result", l.PatronID)
		}
	}
}

// TestCountActiveAndOverdueLoans pins the two dashboard count queries
// with the same fixture: their subsets are disjoint (active excludes overdue)
// so the two cards on the dashboard never double-count the same loan.
func TestCountActiveAndOverdueLoans(t *testing.T) {
	dm := setupTestDB(t)
	bookA := mustCreateBook(t, dm, "Book A", 1)
	bookB := mustCreateBook(t, dm, "Book B", 1)
	bookC := mustCreateBook(t, dm, "Book C", 1)
	bookD := mustCreateBook(t, dm, "Book D", 1)
	patronID := mustCreatePatron(t, dm, "Pat")

	nextWeek := time.Now().AddDate(0, 0, 7).UTC().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).UTC().Format("2006-01-02")

	mustInsertLoan(t, dm, bookA, patronID, nextWeek, "")                    // active (not overdue)
	mustInsertLoan(t, dm, bookB, patronID, yesterday, "")                   // overdue (excluded from active)
	mustInsertLoan(t, dm, bookC, patronID, nextWeek, "2026-04-01 12:00:00") // returned
	mustInsertLoan(t, dm, bookD, patronID, yesterday, "2026-04-01 12:00:00") // returned past due date

	active, err := dm.CountActiveLoans()
	if err != nil {
		t.Fatalf("CountActiveLoans: %v", err)
	}
	if active != 1 {
		t.Errorf("expected 1 active (unreturned, not overdue), got %d", active)
	}

	overdue, err := dm.CountOverdueLoans()
	if err != nil {
		t.Fatalf("CountOverdueLoans: %v", err)
	}
	if overdue != 1 {
		t.Errorf("expected 1 overdue, got %d", overdue)
	}
}

// TestCountOutOfStockReflectsBooks pins that CountOutOfStock queries the
// books table, not the loans table. Regression guard against the bug
// caught during session 2 review where the query pointed at loans.
func TestCountOutOfStockReflectsBooks(t *testing.T) {
	dm := setupTestDB(t)

	// Seed three books: two with zero available, one with stock.
	outA := &Book{Title: "Zero A", QuantityTotal: 1, QuantityAvailable: 0}
	if _, err := dm.CreateBook(outA, []string{"X"}); err != nil {
		t.Fatalf("CreateBook outA: %v", err)
	}
	outB := &Book{Title: "Zero B", QuantityTotal: 2, QuantityAvailable: 0}
	if _, err := dm.CreateBook(outB, []string{"Y"}); err != nil {
		t.Fatalf("CreateBook outB: %v", err)
	}
	inStock := &Book{Title: "Has Stock", QuantityTotal: 3, QuantityAvailable: 3}
	if _, err := dm.CreateBook(inStock, []string{"Z"}); err != nil {
		t.Fatalf("CreateBook inStock: %v", err)
	}

	count, err := dm.CountOutOfStock()
	if err != nil {
		t.Fatalf("CountOutOfStock: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 out-of-stock books, got %d", count)
	}
}
