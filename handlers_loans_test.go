package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// seedLoanFixtureBook inserts a book by title with the given quantity
// and returns its id. Wraps mustCreateBook (defined in db_loans_test.go)
// so handler tests don't repeat the boilerplate.
func seedLoanFixtureBook(t *testing.T, dm *DatabaseManager, title string, qty int) int {
	t.Helper()
	return mustCreateBook(t, dm, title, qty)
}

// seedLoanFixturePatron inserts a patron and returns its id.
func seedLoanFixturePatron(t *testing.T, dm *DatabaseManager, name string) int {
	t.Helper()
	return mustCreatePatron(t, dm, name)
}

// flashCode reads the slug stored in the named flash cookie. Returns
// the empty string if the cookie isn't set or has been cleared.
func flashCode(rr *httptest.ResponseRecorder, name string) string {
	v, _ := flashSet(rr, name)
	return v
}

// ---------- HandleCheckout ----------

// TestCheckoutHappyPath pins POST /books/:id/checkout: redirect to
// /books/:id, success flash with code loan_checkout_success, loan row
// created, quantity_available decremented.
func TestCheckoutHappyPath(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	bookID := seedLoanFixtureBook(t, dm, "Checkout Target", 2)
	patronID := seedLoanFixturePatron(t, dm, "Checkout Patron")

	rr := postStaffForm(t, router, fmt.Sprintf("/books/%d/checkout", bookID), sess, csrf, map[string]string{
		"patron_id": fmt.Sprintf("%d", patronID),
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d. body: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != fmt.Sprintf("/books/%d", bookID) {
		t.Errorf("expected redirect to /books/%d, got %q", bookID, loc)
	}
	if got := flashCode(rr, "flash_success"); got != "loan_checkout_success" {
		t.Errorf("expected flash_success=loan_checkout_success, got %q", got)
	}

	var count, available int
	if err := dm.db.QueryRow(
		`SELECT COUNT(*) FROM loans WHERE book_id = ? AND patron_id = ? AND returned_at IS NULL`,
		bookID, patronID).Scan(&count); err != nil {
		t.Fatalf("count loans: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 active loan, got %d", count)
	}
	if err := dm.db.QueryRow(`SELECT quantity_available FROM books WHERE id = ?`, bookID).Scan(&available); err != nil {
		t.Fatalf("query quantity: %v", err)
	}
	if available != 1 {
		t.Errorf("expected quantity_available=1, got %d", available)
	}
}

// TestCheckoutMissingPatronID pins the validation guard: missing or
// empty patron_id flashes loan_patron_required and redirects without
// creating a loan.
func TestCheckoutMissingPatronID(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	bookID := seedLoanFixtureBook(t, dm, "Lonely Book", 1)

	rr := postStaffForm(t, router, fmt.Sprintf("/books/%d/checkout", bookID), sess, csrf, map[string]string{
		"patron_id": "",
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if got := flashCode(rr, "flash_error"); got != "loan_patron_required" {
		t.Errorf("expected flash_error=loan_patron_required, got %q", got)
	}

	var count int
	if err := dm.db.QueryRow(`SELECT COUNT(*) FROM loans WHERE book_id = ?`, bookID).Scan(&count); err != nil {
		t.Fatalf("count loans: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 loans on validation failure, got %d", count)
	}
}

// TestCheckoutBlockedByOverdue pins the ErrPatronHasOverdue -> flash
// mapping. Without the mapping, the guard sentinel would surface as a
// 500 instead of a banner.
func TestCheckoutBlockedByOverdue(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	overdueBook := seedLoanFixtureBook(t, dm, "Overdue Holder", 1)
	wantedBook := seedLoanFixtureBook(t, dm, "Wanted Title", 1)
	patronID := seedLoanFixturePatron(t, dm, "Has Overdue")

	yesterday := time.Now().AddDate(0, 0, -1).UTC().Format("2006-01-02")
	mustInsertLoan(t, dm, overdueBook, patronID, yesterday, "")

	rr := postStaffForm(t, router, fmt.Sprintf("/books/%d/checkout", wantedBook), sess, csrf, map[string]string{
		"patron_id": fmt.Sprintf("%d", patronID),
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if got := flashCode(rr, "flash_error"); got != "loan_blocked_overdue" {
		t.Errorf("expected flash_error=loan_blocked_overdue, got %q", got)
	}
}

// TestCheckoutBlockedByLimit pins the ErrPatronAtLoanLimit -> flash
// mapping. Patron at the cap cannot check out more.
func TestCheckoutBlockedByLimit(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	patronID := seedLoanFixturePatron(t, dm, "At Limit")
	nextWeek := time.Now().AddDate(0, 0, 7).UTC().Format("2006-01-02")

	for i := range MaxActiveLoansPerPatron {
		fillerID := seedLoanFixtureBook(t, dm, fmt.Sprintf("Filler %d", i), 1)
		mustInsertLoan(t, dm, fillerID, patronID, nextWeek, "")
	}

	straw := seedLoanFixtureBook(t, dm, "The Straw", 1)
	rr := postStaffForm(t, router, fmt.Sprintf("/books/%d/checkout", straw), sess, csrf, map[string]string{
		"patron_id": fmt.Sprintf("%d", patronID),
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if got := flashCode(rr, "flash_error"); got != "loan_blocked_limit" {
		t.Errorf("expected flash_error=loan_blocked_limit, got %q", got)
	}
}

// TestCheckoutNoCopiesAvailable pins the ErrNoCopiesAvailable -> flash
// mapping for a book with quantity_available = 0.
func TestCheckoutNoCopiesAvailable(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	bookID := seedLoanFixtureBook(t, dm, "Out of Stock", 0)
	patronID := seedLoanFixturePatron(t, dm, "Eager Patron")

	rr := postStaffForm(t, router, fmt.Sprintf("/books/%d/checkout", bookID), sess, csrf, map[string]string{
		"patron_id": fmt.Sprintf("%d", patronID),
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if got := flashCode(rr, "flash_error"); got != "loan_no_copies" {
		t.Errorf("expected flash_error=loan_no_copies, got %q", got)
	}
}

// TestCheckoutBookNotFound pins the 404 path: a checkout to a
// nonexistent book id renders the error page, not a 500 from a guard.
func TestCheckoutBookNotFound(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	patronID := seedLoanFixturePatron(t, dm, "Patron")

	rr := postStaffForm(t, router, "/books/99999/checkout", sess, csrf, map[string]string{
		"patron_id": fmt.Sprintf("%d", patronID),
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// TestCheckoutNonNumericBookID pins the 404 path for a malformed URL
// segment.
func TestCheckoutNonNumericBookID(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	rr := postStaffForm(t, router, "/books/abc/checkout", sess, csrf, map[string]string{
		"patron_id": "1",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// ---------- HandleReturn ----------

// TestReturnHappyPath pins POST /loans/:id/return: redirect to /loans,
// success flash, returned_at is set, quantity_available is restored.
func TestReturnHappyPath(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	bookID := seedLoanFixtureBook(t, dm, "Return Target", 1)
	patronID := seedLoanFixturePatron(t, dm, "Returnee")

	if err := dm.CheckoutBook(bookID, patronID, time.Now().AddDate(0, 0, DefaultLoanTermDays)); err != nil {
		t.Fatalf("CheckoutBook setup: %v", err)
	}
	var loanID int
	if err := dm.db.QueryRow(`SELECT id FROM loans WHERE book_id = ?`, bookID).Scan(&loanID); err != nil {
		t.Fatalf("query loan id: %v", err)
	}

	rr := postStaffForm(t, router, fmt.Sprintf("/loans/%d/return", loanID), sess, csrf, nil)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d. body: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/loans" {
		t.Errorf("expected redirect to /loans, got %q", loc)
	}
	if got := flashCode(rr, "flash_success"); got != "loan_return_success" {
		t.Errorf("expected flash_success=loan_return_success, got %q", got)
	}

	var returnedAt sql.NullString
	if err := dm.db.QueryRow(`SELECT returned_at FROM loans WHERE id = ?`, loanID).Scan(&returnedAt); err != nil {
		t.Fatalf("query returned_at: %v", err)
	}
	if !returnedAt.Valid {
		t.Errorf("expected returned_at set after return")
	}

	var available int
	if err := dm.db.QueryRow(`SELECT quantity_available FROM books WHERE id = ?`, bookID).Scan(&available); err != nil {
		t.Fatalf("query quantity: %v", err)
	}
	if available != 1 {
		t.Errorf("expected quantity_available=1 after return, got %d", available)
	}
}

// TestReturnAlreadyReturned pins the idempotency banner: the second
// return on the same loan flashes loan_already_returned without a 500.
func TestReturnAlreadyReturned(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	bookID := seedLoanFixtureBook(t, dm, "Twice Returned", 1)
	patronID := seedLoanFixturePatron(t, dm, "Quick Returner")

	if err := dm.CheckoutBook(bookID, patronID, time.Now().AddDate(0, 0, DefaultLoanTermDays)); err != nil {
		t.Fatalf("CheckoutBook setup: %v", err)
	}
	var loanID int
	if err := dm.db.QueryRow(`SELECT id FROM loans WHERE book_id = ?`, bookID).Scan(&loanID); err != nil {
		t.Fatalf("query loan id: %v", err)
	}
	if err := dm.ReturnBook(loanID); err != nil {
		t.Fatalf("first ReturnBook setup: %v", err)
	}

	rr := postStaffForm(t, router, fmt.Sprintf("/loans/%d/return", loanID), sess, csrf, nil)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if got := flashCode(rr, "flash_error"); got != "loan_already_returned" {
		t.Errorf("expected flash_error=loan_already_returned, got %q", got)
	}
}

// TestReturnLoanNotFound pins the 404 path for a bogus loan id.
func TestReturnLoanNotFound(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	rr := postStaffForm(t, router, "/loans/99999/return", sess, csrf, nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// ---------- HandleLoansList ----------

// TestLoansListDefaultsToActive pins GET /loans without a filter param:
// renders the active view (200 OK + page heading present).
func TestLoansListDefaultsToActive(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin", "admin")

	req, _ := http.NewRequest("GET", "/loans", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Loans") {
		t.Errorf("expected 'Loans' heading in body")
	}
}

// TestLoansListActiveFilterShowsActiveOnly pins ?filter=active: only
// non-returned, not-past-due loans appear.
func TestLoansListActiveFilterShowsActiveOnly(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin", "admin")
	bookA := seedLoanFixtureBook(t, dm, "Active Book Title", 1)
	bookB := seedLoanFixtureBook(t, dm, "Overdue Book Title", 1)
	patronID := seedLoanFixturePatron(t, dm, "Pat")

	nextWeek := time.Now().AddDate(0, 0, 7).UTC().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).UTC().Format("2006-01-02")
	mustInsertLoan(t, dm, bookA, patronID, nextWeek, "")
	mustInsertLoan(t, dm, bookB, patronID, yesterday, "")

	req, _ := http.NewRequest("GET", "/loans?filter=active", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Active Book Title") {
		t.Errorf("expected active loan in active view")
	}
	if strings.Contains(body, "Overdue Book Title") {
		t.Errorf("did not expect overdue loan in active view")
	}
}

// TestLoansListOverdueFilterShowsOverdueOnly pins ?filter=overdue: only
// past-due, non-returned loans appear.
func TestLoansListOverdueFilterShowsOverdueOnly(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin", "admin")
	bookA := seedLoanFixtureBook(t, dm, "Active Book Title", 1)
	bookB := seedLoanFixtureBook(t, dm, "Overdue Book Title", 1)
	patronID := seedLoanFixturePatron(t, dm, "Pat")

	nextWeek := time.Now().AddDate(0, 0, 7).UTC().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).UTC().Format("2006-01-02")
	mustInsertLoan(t, dm, bookA, patronID, nextWeek, "")
	mustInsertLoan(t, dm, bookB, patronID, yesterday, "")

	req, _ := http.NewRequest("GET", "/loans?filter=overdue", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, "Active Book Title") {
		t.Errorf("did not expect active loan in overdue view")
	}
	if !strings.Contains(body, "Overdue Book Title") {
		t.Errorf("expected overdue loan in overdue view")
	}
}

// TestLoansListInvalidFilterFallsThroughToActive pins the lenient
// default: a typo'd filter does not 404, it renders active view.
func TestLoansListInvalidFilterFallsThroughToActive(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin", "admin")

	req, _ := http.NewRequest("GET", "/loans?filter=garbage", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 fallthrough, got %d", rr.Code)
	}
}
