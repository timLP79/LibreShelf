package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// postStaffForm POSTs a urlencoded form to path with the given session
// cookie and CSRF token attached. Used by every mutation test below to
// keep request construction out of the test bodies.
func postStaffForm(t *testing.T, router *gin.Engine, path string, sess *http.Cookie, csrf string, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("csrf_token", csrf)
	for k, v := range fields {
		form.Set(k, v)
	}
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// validStaffFields returns a fresh set of valid form fields for the
// create handler. Tests copy and mutate individual fields to exercise
// single-field validation failures without re-spelling the whole form.
func validStaffFields() map[string]string {
	return map[string]string{
		"username":         "new_staff",
		"password":         "ValidPass1!",
		"password_confirm": "ValidPass1!",
		"role":             "staff",
	}
}

// flashSet reports whether the response carries a live flash cookie of
// the given name (non-empty value, positive MaxAge). readAndClearFlash
// uses MaxAge=-1 to clear, so only a just-set cookie will return true.
func flashSet(rr *httptest.ResponseRecorder, name string) (string, bool) {
	for _, c := range rr.Result().Cookies() {
		if c.Name == name && c.Value != "" && c.MaxAge > 0 {
			return c.Value, true
		}
	}
	return "", false
}

func assertFlashErrorSet(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if _, ok := flashSet(rr, "flash_error"); !ok {
		t.Errorf("expected flash_error cookie to be set, got cookies=%v", rr.Result().Cookies())
	}
}

func assertFlashSuccessSet(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if _, ok := flashSet(rr, "flash_success"); !ok {
		t.Errorf("expected flash_success cookie to be set, got cookies=%v", rr.Result().Cookies())
	}
}

func assertNoFlashError(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if v, ok := flashSet(rr, "flash_error"); ok {
		t.Errorf("unexpected flash_error cookie set: %q", v)
	}
}

// ---------- list handler ----------

// TestStaffListRendersAsAdmin pins the happy path for GET /staff: 200 and
// the page heading is present in the body. Protects against a future edit
// accidentally routing /staff to the wrong template or dropping the handler.
func TestStaffListRendersAsAdmin(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin", "admin")

	req, _ := http.NewRequest("GET", "/staff", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Staff Management") {
		t.Errorf("expected body to contain 'Staff Management'")
	}
}

// ---------- create handler ----------

// TestStaffCreateHappyPath verifies a valid POST /staff inserts the user
// and redirects. Confirms the end-to-end path: form parse, validation,
// bcrypt, CreateUser, PRG redirect to /staff.
func TestStaffCreateHappyPath(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	rr := postStaffForm(t, router, "/staff", sess, csrf, validStaffFields())

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d. body: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); !strings.HasPrefix(loc, "/staff") {
		t.Errorf("expected redirect to /staff, got %q", loc)
	}
	assertNoFlashError(t, rr)

	user, err := dm.GetUserByUsername("new_staff")
	if err != nil {
		t.Fatalf("expected user new_staff to exist after create, got %v", err)
	}
	if user.Role != "staff" {
		t.Errorf("expected role 'staff', got %q", user.Role)
	}
}

// TestStaffCreateRejectsPasswordMismatch verifies the handler rejects
// when password and password_confirm differ. The user must NOT be created.
func TestStaffCreateRejectsPasswordMismatch(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	fields := validStaffFields()
	fields["password_confirm"] = "DifferentPass1!"

	rr := postStaffForm(t, router, "/staff", sess, csrf, fields)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)
	if _, err := dm.GetUserByUsername("new_staff"); err != sql.ErrNoRows {
		t.Errorf("user should not have been created on password mismatch, GetUserByUsername returned %v", err)
	}
}

// TestStaffCreateRejectsWeakPassword verifies ValidatePassword is enforced
// in the handler. "weakpass" misses uppercase, digit, and special.
func TestStaffCreateRejectsWeakPassword(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	fields := validStaffFields()
	fields["password"] = "weakpass"
	fields["password_confirm"] = "weakpass"

	rr := postStaffForm(t, router, "/staff", sess, csrf, fields)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)
	if _, err := dm.GetUserByUsername("new_staff"); err != sql.ErrNoRows {
		t.Errorf("user should not have been created on weak password, got %v", err)
	}
}

// TestStaffCreateRejectsBadUsername verifies ValidateUsername is enforced
// server-side. Dots and hyphens are rejected by the regex even though a
// stale client-side pattern attribute once allowed them.
func TestStaffCreateRejectsBadUsername(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	fields := validStaffFields()
	fields["username"] = "bad.name"

	rr := postStaffForm(t, router, "/staff", sess, csrf, fields)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)
	if _, err := dm.GetUserByUsername("bad.name"); err != sql.ErrNoRows {
		t.Errorf("user should not have been created on bad username, got %v", err)
	}
}

// TestStaffCreateRejectsInvalidRole verifies the role whitelist rejects
// 'patron'. Without the whitelist, /staff would be an escalation path for
// creating arbitrary-role users.
func TestStaffCreateRejectsInvalidRole(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	fields := validStaffFields()
	fields["role"] = "patron"

	rr := postStaffForm(t, router, "/staff", sess, csrf, fields)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)
	if _, err := dm.GetUserByUsername("new_staff"); err != sql.ErrNoRows {
		t.Errorf("user should not have been created on invalid role, got %v", err)
	}
}

// TestStaffCreateRejectsDuplicateUsername verifies the UNIQUE constraint
// error from CreateUser surfaces as a PRG error rather than a 500.
func TestStaffCreateRejectsDuplicateUsername(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	// Seed an existing user, then try to POST with the same username.
	mustCreateUser(t, dm, "taken", "staff")

	fields := validStaffFields()
	fields["username"] = "taken"

	rr := postStaffForm(t, router, "/staff", sess, csrf, fields)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)

	// Exactly one row with username "taken" should remain (the seed).
	staff, err := dm.GetAllStaff()
	if err != nil {
		t.Fatalf("GetAllStaff: %v", err)
	}
	takenCount := 0
	for _, s := range staff {
		if s.Username == "taken" {
			takenCount++
		}
	}
	if takenCount != 1 {
		t.Errorf("expected exactly 1 row with username 'taken', got %d", takenCount)
	}
}

// ---------- edit handler ----------

// TestStaffEditHappyPath verifies a combined username + role update applies
// both fields in a single edit (DEC-020). The PRG redirect lands back at
// /staff with no error= query param.
func TestStaffEditHappyPath(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	victimID := mustCreateUser(t, dm, "target_staff", "staff")

	path := fmt.Sprintf("/staff/%d/edit", victimID)
	rr := postStaffForm(t, router, path, sess, csrf, map[string]string{
		"username": "promoted_admin",
		"role":     "admin",
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertNoFlashError(t, rr)

	user, err := dm.GetUserByID(victimID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.Username != "promoted_admin" {
		t.Errorf("expected username 'promoted_admin', got %q", user.Username)
	}
	if user.Role != "admin" {
		t.Errorf("expected role 'admin', got %q", user.Role)
	}
}

// TestStaffEditRejectsSelfDemote verifies the handler refuses to demote
// the acting admin. Uses a two-admin setup so the last-admin guard is NOT
// also firing; this isolates the self-demote guard.
func TestStaffEditRejectsSelfDemote(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	// Second admin so CountAdmins > 1 and the last-admin guard does not fire.
	mustCreateUser(t, dm, "admin2", "admin")

	actor, err := dm.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin): %v", err)
	}

	path := fmt.Sprintf("/staff/%d/edit", actor.ID)
	rr := postStaffForm(t, router, path, sess, csrf, map[string]string{
		"username": actor.Username,
		"role":     "staff",
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)

	after, err := dm.GetUserByID(actor.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if after.Role != "admin" {
		t.Errorf("self-demote should have been rejected, role is now %q", after.Role)
	}
}

// TestStaffEditRejectsLastAdminDemote verifies the last-admin guard. When
// only one admin exists in the DB, any attempt to demote them (even via
// self edit) must be rejected. On an admin-only route, target==actor is
// the only reachable scenario, so self-demote may also fire here -- the
// assertion is "no mutation applied" regardless of which guard wins.
func TestStaffEditRejectsLastAdminDemote(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	actor, err := dm.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin): %v", err)
	}
	// Sanity: exactly one admin in the DB so the last-admin guard is the
	// one we want to trip.
	if count, _ := dm.CountAdmins(); count != 1 {
		t.Fatalf("expected 1 admin at start, got %d", count)
	}

	path := fmt.Sprintf("/staff/%d/edit", actor.ID)
	rr := postStaffForm(t, router, path, sess, csrf, map[string]string{
		"username": actor.Username,
		"role":     "staff",
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)

	after, err := dm.GetUserByID(actor.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if after.Role != "admin" {
		t.Errorf("last-admin demote should have been rejected, role is now %q", after.Role)
	}
	if count, _ := dm.CountAdmins(); count != 1 {
		t.Errorf("admin count should still be 1 after rejected demote, got %d", count)
	}
}

// TestStaffEditRejectsInvalidRole verifies the edit handler's role
// whitelist, parallel to the create handler's. 'patron' must be rejected.
func TestStaffEditRejectsInvalidRole(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	victimID := mustCreateUser(t, dm, "victim_staff", "staff")

	path := fmt.Sprintf("/staff/%d/edit", victimID)
	rr := postStaffForm(t, router, path, sess, csrf, map[string]string{
		"username": "victim_staff",
		"role":     "patron",
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)

	after, err := dm.GetUserByID(victimID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if after.Role != "staff" {
		t.Errorf("role should still be 'staff', got %q", after.Role)
	}
}

// TestStaffEditRejectsPatronTarget is the IDOR guard: /staff/:id/edit
// must refuse when :id resolves to a patron user row. Expected status is
// 404 (pretend the target does not exist inside the /staff scope).
func TestStaffEditRejectsPatronTarget(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	patronID := mustCreateUser(t, dm, "patron_victim", "patron")

	path := fmt.Sprintf("/staff/%d/edit", patronID)
	rr := postStaffForm(t, router, path, sess, csrf, map[string]string{
		"username": "patron_victim",
		"role":     "staff",
	})

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for patron target on edit, got %d. body: %s", rr.Code, rr.Body.String())
	}

	after, err := dm.GetUserByID(patronID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if after.Role != "patron" {
		t.Errorf("patron role must not be mutated via /staff edit, got %q", after.Role)
	}
	if after.Username != "patron_victim" {
		t.Errorf("patron username must not be mutated via /staff edit, got %q", after.Username)
	}
}

// ---------- delete handler ----------

// TestStaffDeleteHappyPath verifies POST /staff/:id/delete removes both
// the user row AND any live sessions for that user in a single transaction
// (DEC-022). Without the session wipe, foreign keys would make the delete
// fail silently or leak stale session rows.
func TestStaffDeleteHappyPath(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	victimID := mustCreateUser(t, dm, "doomed", "staff")

	// Seed a live session for the victim so the transactional delete has
	// something real to clean up.
	if err := dm.CreateSession("victim-sess", victimID, "victim-csrf", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession (victim): %v", err)
	}

	path := fmt.Sprintf("/staff/%d/delete", victimID)
	rr := postStaffForm(t, router, path, sess, csrf, nil)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertNoFlashError(t, rr)

	if _, err := dm.GetUserByID(victimID); err != sql.ErrNoRows {
		t.Errorf("expected user deleted, got %v", err)
	}
	if _, err := dm.GetSession("victim-sess"); err != sql.ErrNoRows {
		t.Errorf("expected victim session deleted, got %v", err)
	}
}

// TestStaffDeleteRejectsSelf verifies the handler refuses to delete the
// acting admin. Uses a two-admin setup so the last-admin guard does NOT
// also fire; this isolates the self-delete guard.
func TestStaffDeleteRejectsSelf(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	mustCreateUser(t, dm, "admin2", "admin")

	actor, err := dm.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin): %v", err)
	}

	path := fmt.Sprintf("/staff/%d/delete", actor.ID)
	rr := postStaffForm(t, router, path, sess, csrf, nil)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)

	if _, err := dm.GetUserByID(actor.ID); err != nil {
		t.Errorf("self-delete should have been rejected, but actor is gone: %v", err)
	}
}

// TestStaffDeleteRejectsLastAdmin verifies the last-admin guard on delete.
// With one admin, target==actor on an admin-only route, so self-delete may
// also fire; the assertion is "no deletion occurred" regardless of which
// guard wins.
func TestStaffDeleteRejectsLastAdmin(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	actor, err := dm.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin): %v", err)
	}
	if count, _ := dm.CountAdmins(); count != 1 {
		t.Fatalf("expected 1 admin at start, got %d", count)
	}

	path := fmt.Sprintf("/staff/%d/delete", actor.ID)
	rr := postStaffForm(t, router, path, sess, csrf, nil)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)

	if _, err := dm.GetUserByID(actor.ID); err != nil {
		t.Errorf("last-admin delete should have been rejected, but actor is gone: %v", err)
	}
	if count, _ := dm.CountAdmins(); count != 1 {
		t.Errorf("admin count should still be 1 after rejected delete, got %d", count)
	}
}

// TestStaffDeleteRejectsPatronTarget is the IDOR guard on delete: a
// patron id posted to /staff/:id/delete must be refused with 404, and
// the patron row must remain intact.
func TestStaffDeleteRejectsPatronTarget(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	patronID := mustCreateUser(t, dm, "patron_victim", "patron")

	path := fmt.Sprintf("/staff/%d/delete", patronID)
	rr := postStaffForm(t, router, path, sess, csrf, nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for patron target on delete, got %d. body: %s", rr.Code, rr.Body.String())
	}

	after, err := dm.GetUserByID(patronID)
	if err != nil {
		t.Fatalf("patron should still exist after rejected delete, got %v", err)
	}
	if after.Role != "patron" {
		t.Errorf("patron role mutated unexpectedly, got %q", after.Role)
	}
}

// ---------- reset-password handler ----------

// TestStaffResetPasswordHappyPath verifies a successful reset: the new
// password hash is persisted AND the target user's live sessions are
// wiped (per DEC-022 transactional design). Pins both halves of the
// transactional write in a single test.
func TestStaffResetPasswordHappyPath(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	victimID := mustCreateUser(t, dm, "victim", "staff")

	if err := dm.CreateSession("victim-sess", victimID, "victim-csrf", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession (victim): %v", err)
	}

	const newPassword = "FreshPass1!"
	path := fmt.Sprintf("/staff/%d/password", victimID)
	rr := postStaffForm(t, router, path, sess, csrf, map[string]string{
		"password":         newPassword,
		"password_confirm": newPassword,
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertNoFlashError(t, rr)
	assertFlashSuccessSet(t, rr)

	victim, err := dm.GetUserByID(victimID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(victim.PasswordHash), []byte(newPassword)); err != nil {
		t.Errorf("new password does not hash-match stored hash: %v", err)
	}

	if _, err := dm.GetSession("victim-sess"); err != sql.ErrNoRows {
		t.Errorf("expected victim session wiped after reset, got %v", err)
	}
}

// TestStaffResetPasswordRejectsPasswordMismatch verifies the match check
// fires before any hash write. Target's stored hash must be unchanged.
func TestStaffResetPasswordRejectsPasswordMismatch(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	victimID := mustCreateUser(t, dm, "victim", "staff")

	before, err := dm.GetUserByID(victimID)
	if err != nil {
		t.Fatalf("GetUserByID (before): %v", err)
	}

	path := fmt.Sprintf("/staff/%d/password", victimID)
	rr := postStaffForm(t, router, path, sess, csrf, map[string]string{
		"password":         "FreshPass1!",
		"password_confirm": "DifferentPass1!",
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)

	after, err := dm.GetUserByID(victimID)
	if err != nil {
		t.Fatalf("GetUserByID (after): %v", err)
	}
	if after.PasswordHash != before.PasswordHash {
		t.Errorf("password hash should be unchanged on mismatch")
	}
}

// TestStaffResetPasswordRejectsWeakPassword verifies ValidatePassword is
// enforced on reset. The stored hash must remain unchanged.
func TestStaffResetPasswordRejectsWeakPassword(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	victimID := mustCreateUser(t, dm, "victim", "staff")

	before, err := dm.GetUserByID(victimID)
	if err != nil {
		t.Fatalf("GetUserByID (before): %v", err)
	}

	path := fmt.Sprintf("/staff/%d/password", victimID)
	rr := postStaffForm(t, router, path, sess, csrf, map[string]string{
		"password":         "weakpass",
		"password_confirm": "weakpass",
	})

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 PRG redirect, got %d. body: %s", rr.Code, rr.Body.String())
	}
	assertFlashErrorSet(t, rr)

	after, err := dm.GetUserByID(victimID)
	if err != nil {
		t.Fatalf("GetUserByID (after): %v", err)
	}
	if after.PasswordHash != before.PasswordHash {
		t.Errorf("password hash should be unchanged on weak password")
	}
}

// TestStaffResetPasswordRejectsPatronTarget is the IDOR guard on reset:
// a patron id posted to /staff/:id/password must be refused with 404,
// and the patron's password hash must remain intact.
func TestStaffResetPasswordRejectsPatronTarget(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")
	patronID := mustCreateUser(t, dm, "patron_victim", "patron")

	before, err := dm.GetUserByID(patronID)
	if err != nil {
		t.Fatalf("GetUserByID (before): %v", err)
	}

	path := fmt.Sprintf("/staff/%d/password", patronID)
	rr := postStaffForm(t, router, path, sess, csrf, map[string]string{
		"password":         "FreshPass1!",
		"password_confirm": "FreshPass1!",
	})

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for patron target on reset, got %d. body: %s", rr.Code, rr.Body.String())
	}

	after, err := dm.GetUserByID(patronID)
	if err != nil {
		t.Fatalf("GetUserByID (after): %v", err)
	}
	if after.PasswordHash != before.PasswordHash {
		t.Errorf("patron hash must not be mutated via /staff reset")
	}
}
