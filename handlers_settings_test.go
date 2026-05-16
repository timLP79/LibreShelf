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

func TestSettingsPageGET_RendersLockedToggleWhenEnvLocked(t *testing.T) {
	router, dm := setupTestRouter(t)
	withOfflineEnvDefault(t, true)
	sess, _ := loginAs(t, dm, "admin1", "admin")

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "disabled") {
		t.Errorf("expected 'disabled' attribute in locked toggle, got %q", body)
	}
	if !strings.Contains(body, "LIBRESHELF_OFFLINE") {
		t.Errorf("expected 'LIBRESHELF_OFFLINE' explanation in body, got %q", body)
	}
}

func TestSettingsPagePOST_SkipsOfflineModeWhenEnvLocked(t *testing.T) {
	router, dm := setupTestRouter(t)
	withOfflineEnvDefault(t, true)
	sess, csrf := loginAs(t, dm, "admin1", "admin")

	// Pre-condition: no offline_mode row.
	if v, _ := dm.GetSetting("offline_mode"); v != "" {
		t.Fatalf("pre-condition: offline_mode row should be empty, got %q", v)
	}

	// Submit a crafted POST that tries to flip offline_mode to true
	// while the env-var lock is in place. The handler must skip the
	// write regardless of the form value. Also flip the staff toggle
	// so we can confirm OTHER settings still write normally.
	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("offline_mode", "on")
	form.Set("staff_can_import_patrons", "on")
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}

	// Post-condition: offline_mode row still empty (write skipped).
	v, err := dm.GetSetting("offline_mode")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if v != "" {
		t.Errorf("offline_mode row should not be written while locked, got %q", v)
	}

	// Confirm staff_can_import_patrons WAS written (lock affects only offline_mode).
	if !dm.GetSettingBool("staff_can_import_patrons", false) {
		t.Errorf("staff_can_import_patrons should be true (lock only affects offline_mode)")
	}
}

// TestSettingsPagePOST_BothTogglesWrittenEveryPOST documents the design
// of the multi-toggle settings handler: each POST writes every known
// setting from the form, treating an absent checkbox as "off." This
// prevents stale state when forms are partially submitted, and it
// requires the UI to always include every checkbox in every POST.
//
// Why this test exists: when the handler grew from one toggle to two,
// it became possible for a partial form (e.g. only offline_mode=on)
// to silently reset the absent toggle to false. The existing per-toggle
// tests didn't catch this because each starts from a fresh DB. This
// test exercises the cross-toggle behavior explicitly.
func TestSettingsPagePOST_BothTogglesWrittenEveryPOST(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin1", "admin")

	// Phase 1: enable both in a single POST.
	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("staff_can_import_patrons", "on")
	form.Set("offline_mode", "on")
	req := httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("phase 1 status = %d, want 303", rr.Code)
	}
	if !dm.GetSettingBool("staff_can_import_patrons", false) {
		t.Errorf("phase 1: staff_can_import_patrons should be true")
	}
	if !dm.GetSettingBool("offline_mode", false) {
		t.Errorf("phase 1: offline_mode should be true")
	}

	// Phase 2: POST again with both still on. Idempotent.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("phase 2 status = %d, want 303", rr.Code)
	}
	if !dm.GetSettingBool("staff_can_import_patrons", false) || !dm.GetSettingBool("offline_mode", false) {
		t.Errorf("phase 2: both toggles should remain true after repeat POST")
	}

	// Phase 3: POST with staff_can_import_patrons=on but offline_mode absent.
	// Documents the design: absent checkbox = off. Staff toggle stays true,
	// offline toggle flips back to false.
	form = url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("staff_can_import_patrons", "on")
	req = httptest.NewRequest("POST", "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sess)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("phase 3 status = %d, want 303", rr.Code)
	}
	if !dm.GetSettingBool("staff_can_import_patrons", false) {
		t.Errorf("phase 3: staff_can_import_patrons should remain true (was in form)")
	}
	if dm.GetSettingBool("offline_mode", true) {
		t.Errorf("phase 3: offline_mode should be false (absent from form)")
	}
}
