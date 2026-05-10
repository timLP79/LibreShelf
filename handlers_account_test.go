// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestChangePasswordGET_RendersForm(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "alice", "patron")
	user, _ := dm.GetUserByUsername("alice")
	_ = dm.SetMustChangePassword(user.ID)

	req := httptest.NewRequest("GET", "/account/change-password", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Change Password") &&
		!strings.Contains(rr.Body.String(), "Set Password") {
		t.Errorf("expected form text in body")
	}
}

func TestChangePasswordPOST_HappyPathClearsFlagAndRedirectsToLogin(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "alice", "patron")
	user, _ := dm.GetUserByUsername("alice")
	_ = dm.SetMustChangePassword(user.ID)

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("new_password", "NewPass1!")
	form.Set("confirm_password", "NewPass1!")
	req := httptest.NewRequest("POST", "/account/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/login" {
		t.Errorf("Location = %q, want /login", got)
	}

	user, _ = dm.GetUserByUsername("alice")
	if user.MustChangePassword {
		t.Errorf("must_change_password should be cleared after successful change")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("NewPass1!")); err != nil {
		t.Errorf("new password should match stored hash: %v", err)
	}
}

func TestChangePasswordPOST_RejectsWeakPassword(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "alice", "patron")

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("new_password", "abc") // too short, no caps/digit/special
	form.Set("confirm_password", "abc")
	req := httptest.NewRequest("POST", "/account/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (form re-render with error)", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "at least") {
		t.Errorf("expected ValidatePassword error in body")
	}
}

func TestChangePasswordPOST_RejectsMismatchedConfirm(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "alice", "patron")

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("new_password", "NewPass1!")
	form.Set("confirm_password", "OtherPw1!")
	req := httptest.NewRequest("POST", "/account/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (form re-render)", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "do not match") {
		t.Errorf("expected mismatch error in body")
	}
}

func TestChangePasswordPOST_RequiresCSRF(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "alice", "patron")

	form := url.Values{}
	form.Set("new_password", "NewPass1!")
	form.Set("confirm_password", "NewPass1!")
	// deliberately no csrf_token
	req := httptest.NewRequest("POST", "/account/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (CSRF check)", rr.Code)
	}
}
