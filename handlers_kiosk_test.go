package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestKioskAnonymousReturns200 pins the public-by-design contract: no
// session cookie, no redirect, full catalog grid renders.
func TestKioskAnonymousReturns200(t *testing.T) {
	router, _ := setupTestRouter(t)

	req, _ := http.NewRequest("GET", "/kiosk", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Library Catalog") {
		t.Errorf("expected kiosk header in body")
	}
	// SeedBooks puts well-known titles in the test DB; one is enough to
	// confirm the grid actually rendered the books slice rather than an
	// empty page.
	if !strings.Contains(body, "Pride and Prejudice") {
		t.Errorf("expected a seeded book to appear in the kiosk grid")
	}
}

// TestKioskShellHidesStaffNav pins that the kiosk layout cannot leak
// staff navigation, even by accident, regardless of who is hitting it.
// The sidebar links Patrons / Admin / Staff exist on the main layout
// but must not appear under kiosk_layout. This is the test that would
// catch a future refactor that consolidates layouts.
func TestKioskShellHidesStaffNav(t *testing.T) {
	router, _ := setupTestRouter(t)

	req, _ := http.NewRequest("GET", "/kiosk", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	body := rr.Body.String()
	for _, leak := range []string{
		`href="/patrons"`,
		`href="/admin"`,
		`href="/staff"`,
		`href="/logout"`,
		"Sign Out",
	} {
		if strings.Contains(body, leak) {
			t.Errorf("kiosk shell must not contain %q", leak)
		}
	}
}

// TestKioskBookDetailAnonymous200 pins that a kiosk visitor can click
// a book card without bouncing through /login. The auth-gated /books/:id
// is unaffected (covered separately in TestRegressionAnonymousBookDetail).
func TestKioskBookDetailAnonymous200(t *testing.T) {
	router, dm := setupTestRouter(t)
	bookID := mustCreateBook(t, dm, "Kiosk Detail Target", 1)

	req, _ := http.NewRequest("GET", "/kiosk/books/"+strconv.Itoa(bookID), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Kiosk Detail Target") {
		t.Errorf("expected book title to render on kiosk detail page")
	}
	// Public detail must NOT show staff-only widgets.
	for _, leak := range []string{
		"Check Out Book",
		"Loan History",
		"Edit",
		"Delete",
		`name="patron_id"`,
	} {
		if strings.Contains(body, leak) {
			t.Errorf("kiosk book detail must not contain staff widget %q", leak)
		}
	}
}

// TestKioskBookDetailInvalidIDRedirects pins that a non-int id does not
// 500 or render a confused page; it sends the user back to the kiosk
// catalog where they can pick again.
func TestKioskBookDetailInvalidIDRedirects(t *testing.T) {
	router, _ := setupTestRouter(t)

	req, _ := http.NewRequest("GET", "/kiosk/books/not-a-number", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/kiosk" {
		t.Errorf("expected redirect to /kiosk, got %q", loc)
	}
}

// TestKioskBookDetailNotFoundRedirects pins the same redirect for an
// id that parses but doesn't match any row.
func TestKioskBookDetailNotFoundRedirects(t *testing.T) {
	router, _ := setupTestRouter(t)

	req, _ := http.NewRequest("GET", "/kiosk/books/999999", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/kiosk" {
		t.Errorf("expected redirect to /kiosk, got %q", loc)
	}
}

// TestRegressionAnonymousBookDetailGated pins that adding the public
// /kiosk/books/:id route did not accidentally open up the auth-gated
// /books/:id route. An anon hit on /books/:id must still bounce to
// /login. This is the boundary that protects the staff-only checkout
// form, loan history, and patron list on the original detail page.
func TestRegressionAnonymousBookDetailGated(t *testing.T) {
	router, dm := setupTestRouter(t)
	bookID := mustCreateBook(t, dm, "Auth Gated Book", 1)

	req, _ := http.NewRequest("GET", "/books/"+strconv.Itoa(bookID), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected 302 redirect for anon /books/:id, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

