// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSettingsPageGET_AdminCanView(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin1", "admin")

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("admin status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "staff_can_import_patrons") {
		t.Errorf("expected toggle name in body")
	}
}

func TestSettingsPageGET_StaffForbidden(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "staff1", "staff")

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("staff status = %d, want 403", rr.Code)
	}
}

func TestSettingsPagePOST_FlipsToggleOn(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin1", "admin")

	if dm.GetSettingBool("staff_can_import_patrons", false) {
		t.Fatalf("toggle should default off")
	}

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("staff_can_import_patrons", "on")
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if !dm.GetSettingBool("staff_can_import_patrons", false) {
		t.Errorf("toggle should be true after POST with checkbox=on")
	}
}

func TestSettingsPagePOST_FlipsToggleOff(t *testing.T) {
	router, dm := setupTestRouter(t)
	adminID := mustCreateUser(t, dm, "admin_x", "admin")
	_ = dm.SetSetting("staff_can_import_patrons", "true", adminID)
	sess, csrf := loginAs(t, dm, "admin1", "admin")

	form := url.Values{}
	form.Set("csrf_token", csrf)
	// absent checkbox = off
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if dm.GetSettingBool("staff_can_import_patrons", true) {
		t.Errorf("toggle should be false after POST without the checkbox")
	}
}

func TestSettingsPagePOST_FlipsOfflineModeOn(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin1", "admin")

	if dm.GetSettingBool("offline_mode", false) {
		t.Fatalf("offline_mode should default off")
	}

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("offline_mode", "on")
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if !dm.GetSettingBool("offline_mode", false) {
		t.Errorf("offline_mode should be true after POST with checkbox=on")
	}
}

func TestSettingsPagePOST_FlipsOfflineModeOff(t *testing.T) {
	router, dm := setupTestRouter(t)
	adminID := mustCreateUser(t, dm, "admin_off_init", "admin")
	_ = dm.SetSetting("offline_mode", "true", adminID)
	sess, csrf := loginAs(t, dm, "admin1", "admin")

	form := url.Values{}
	form.Set("csrf_token", csrf)
	// offline_mode absent = off
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if dm.GetSettingBool("offline_mode", true) {
		t.Errorf("offline_mode should be false after POST with checkbox absent")
	}
}

func TestSettingsPageGET_RendersOfflineToggle(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin1", "admin")

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "offline_mode") {
		t.Errorf("expected offline_mode toggle in body, got %q", rr.Body.String())
	}
}

func TestSettingsPagePOST_StaffForbidden(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "staff1", "staff")

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("staff_can_import_patrons", "on")
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("staff status = %d, want 403", rr.Code)
	}
	if dm.GetSettingBool("staff_can_import_patrons", false) {
		t.Errorf("toggle should remain off after 403")
	}
}
