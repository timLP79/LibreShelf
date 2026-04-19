package main

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// setupTestDB builds a fresh DatabaseManager against a temporary SQLite
// file. The temp directory is cleaned up automatically when the test
// finishes.
func setupTestDB(t *testing.T) *DatabaseManager {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "libreshelf-db-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	return NewDatabaseManager(tmpDir + "/test.sqlite")
}

// mustCreateUser is a test helper that inserts a user with a known
// bcrypt-hashed password and returns the row's id. Tests that only care
// about a user existing use this to skip the hash-and-fetch boilerplate.
func mustCreateUser(t *testing.T, dm *DatabaseManager, username, role string) int {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("Pw123456!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	if err := dm.CreateUser(username, string(hash), role, nil); err != nil {
		t.Fatalf("CreateUser(%q, %q): %v", username, role, err)
	}
	user, err := dm.GetUserByUsername(username)
	if err != nil {
		t.Fatalf("GetUserByUsername(%q): %v", username, err)
	}
	return user.ID
}

// TestGetAllStaffReturnsOnlyAdminAndStaff verifies patrons are excluded
// from the staff list. A regression where patrons leak in would expose
// patron account existence on the /staff page and blur the role boundary
// established in CP4 (DEC-014).
func TestGetAllStaffReturnsOnlyAdminAndStaff(t *testing.T) {
	dm := setupTestDB(t)
	mustCreateUser(t, dm, "admin1", "admin")
	mustCreateUser(t, dm, "staff_alice", "staff")
	mustCreateUser(t, dm, "patron_bob", "patron")

	staff, err := dm.GetAllStaff()
	if err != nil {
		t.Fatalf("GetAllStaff: %v", err)
	}
	if len(staff) != 2 {
		t.Fatalf("expected 2 staff, got %d", len(staff))
	}
	for _, u := range staff {
		if u.Role == "patron" {
			t.Errorf("patron %q leaked into staff list", u.Username)
		}
	}
}

// TestGetAllStaffOrdering verifies admin rows come before staff rows,
// and within a role usernames are alphabetical. The staff.html template
// relies on this ordering to render the table without its own sort.
func TestGetAllStaffOrdering(t *testing.T) {
	dm := setupTestDB(t)
	mustCreateUser(t, dm, "staff_zach", "staff")
	mustCreateUser(t, dm, "admin_bob", "admin")
	mustCreateUser(t, dm, "staff_alice", "staff")
	mustCreateUser(t, dm, "admin_alice", "admin")

	staff, err := dm.GetAllStaff()
	if err != nil {
		t.Fatalf("GetAllStaff: %v", err)
	}

	expected := []string{"admin_alice", "admin_bob", "staff_alice", "staff_zach"}
	if len(staff) != len(expected) {
		t.Fatalf("expected %d staff, got %d", len(expected), len(staff))
	}
	for i, u := range staff {
		if u.Username != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], u.Username)
		}
	}
}

// TestGetUserByIDHit verifies a round-trip lookup returns the same row
// that CreateUser inserted.
func TestGetUserByIDHit(t *testing.T) {
	dm := setupTestDB(t)
	id := mustCreateUser(t, dm, "admin1", "admin")

	user, err := dm.GetUserByID(id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.Username != "admin1" {
		t.Errorf("expected username admin1, got %q", user.Username)
	}
	if user.Role != "admin" {
		t.Errorf("expected role admin, got %q", user.Role)
	}
}

// TestGetUserByIDMiss verifies that a missing id returns sql.ErrNoRows
// so the caller can distinguish "not found" from "db error" and respond
// with 404 instead of 500.
func TestGetUserByIDMiss(t *testing.T) {
	dm := setupTestDB(t)

	if _, err := dm.GetUserByID(99999); err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// TestUpdateStaffUserUpdates verifies a successful rename + role change
// applies both fields in a single call (DEC-020: combined edit endpoint).
func TestUpdateStaffUserUpdates(t *testing.T) {
	dm := setupTestDB(t)
	id := mustCreateUser(t, dm, "staff1", "staff")

	if err := dm.UpdateStaffUser(id, "renamed", "admin"); err != nil {
		t.Fatalf("UpdateStaffUser: %v", err)
	}

	user, err := dm.GetUserByID(id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.Username != "renamed" {
		t.Errorf("expected username renamed, got %q", user.Username)
	}
	if user.Role != "admin" {
		t.Errorf("expected role admin, got %q", user.Role)
	}
}

// TestUpdateStaffUserRejectsDuplicateUsername verifies the UNIQUE
// constraint on users.username surfaces as an error to the caller. A
// regression here (e.g. swallowed error) would let the handler report
// "success" while the DB silently rejected the write.
func TestUpdateStaffUserRejectsDuplicateUsername(t *testing.T) {
	dm := setupTestDB(t)
	mustCreateUser(t, dm, "taken", "staff")
	id := mustCreateUser(t, dm, "rename_me", "staff")

	if err := dm.UpdateStaffUser(id, "taken", "staff"); err == nil {
		t.Fatal("expected error on duplicate username, got nil")
	}
}

// TestDeleteUserRemovesSessions verifies the transactional DeleteUser
// wipes both the user row and any sessions pointing at it. Without the
// session delete, the foreign-key constraint would reject the user
// delete, so "user still present" is also a regression signal.
func TestDeleteUserRemovesSessions(t *testing.T) {
	dm := setupTestDB(t)
	id := mustCreateUser(t, dm, "soon_gone", "staff")

	if err := dm.CreateSession("tok-test", id, "csrf-test", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := dm.DeleteUser(id); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	if _, err := dm.GetUserByID(id); err != sql.ErrNoRows {
		t.Errorf("expected user deleted (ErrNoRows), got %v", err)
	}
	if _, err := dm.GetSession("tok-test"); err != sql.ErrNoRows {
		t.Errorf("expected session deleted (ErrNoRows), got %v", err)
	}
}

// TestCountAdminsReflectsCurrentState exercises the three operations the
// last-admin guard depends on: creation, demotion, and pure lookup. A
// silent regression here would let the last admin be deleted or demoted
// even though the guard "runs."
func TestCountAdminsReflectsCurrentState(t *testing.T) {
	dm := setupTestDB(t)

	count, err := dm.CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins (empty): %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 admins initially, got %d", count)
	}

	id1 := mustCreateUser(t, dm, "admin1", "admin")
	mustCreateUser(t, dm, "admin2", "admin")
	mustCreateUser(t, dm, "staff1", "staff")

	count, err = dm.CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins (after seeds): %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 admins, got %d", count)
	}

	if err := dm.UpdateStaffUser(id1, "admin1", "staff"); err != nil {
		t.Fatalf("UpdateStaffUser (demote): %v", err)
	}

	count, err = dm.CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins (after demote): %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 admin after demotion, got %d", count)
	}
}
