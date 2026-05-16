// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func multipartImportRequest(t *testing.T, target, csvBody, accountMode, csrf string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("csrf_token", csrf)
	if accountMode != "" {
		_ = w.WriteField("account_mode", accountMode)
	}
	fw, err := w.CreateFormFile("csv_file", "import.csv")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.WriteString(fw, csvBody); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}
	req := httptest.NewRequest("POST", target, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func extractImportToken(t *testing.T, body string) string {
	t.Helper()
	const prefix = `name="token" value="`
	idx := strings.Index(body, prefix)
	if idx == -1 {
		t.Fatalf("token field not found in preview body")
	}
	rest := body[idx+len(prefix):]
	end := strings.Index(rest, `"`)
	if end == -1 {
		t.Fatalf("token field unterminated")
	}
	return rest[:end]
}

func extractDownloadToken(t *testing.T, body, kind string) string {
	t.Helper()
	var marker string
	switch kind {
	case "credentials":
		marker = "Download credentials CSV"
	case "errors":
		marker = "Download error report"
	default:
		t.Fatalf("unknown kind %q", kind)
	}
	markerIdx := strings.Index(body, marker)
	if markerIdx == -1 {
		return ""
	}
	const base = `/admin/patrons/import/download/`
	head := body[:markerIdx]
	urlIdx := strings.LastIndex(head, base)
	if urlIdx == -1 {
		return ""
	}
	rest := body[urlIdx+len(base):]
	end := strings.IndexAny(rest, `"' `)
	if end <= 0 {
		return ""
	}
	return rest[:end]
}

func TestImportForm_AdminGET(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin", "admin")

	req := httptest.NewRequest("GET", "/admin/patrons/import", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Patron Import") {
		t.Errorf("form heading missing")
	}
}

func TestImportPreview_ParsesAndShowsCounts(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	csv := "Name,Email\nAlice Brown,alice@example.com\nBob Wilson,bob@example.com\n,empty@example.com\n"
	req := multipartImportRequest(t, "/admin/patrons/import", csv, "records_only", csrf)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Patron Import: Preview") {
		t.Errorf("expected preview heading")
	}
	if !strings.Contains(body, `name="token"`) {
		t.Errorf("expected hidden token field")
	}
}

func TestImportPreview_RejectsMissingAccountMode(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	csv := "Name\nAlice\n"
	req := multipartImportRequest(t, "/admin/patrons/import", csv, "", csrf)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (form re-render)", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Choose an account mode") {
		t.Errorf("expected error banner")
	}
}

func TestImportCommit_RecordsOnly_CreatesPatronsNoUsers(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	csv := "Name,Email\nAlice Brown,alice@example.com\nBob Wilson,bob@example.com\n"
	req := multipartImportRequest(t, "/admin/patrons/import", csv, "records_only", csrf)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("preview status = %d", rr.Code)
	}
	token := extractImportToken(t, rr.Body.String())

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("token", token)
	commitReq := httptest.NewRequest("POST", "/admin/patrons/import/confirm", strings.NewReader(form.Encode()))
	commitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	commitReq.AddCookie(sess)
	commitRR := httptest.NewRecorder()
	router.ServeHTTP(commitRR, commitReq)

	if commitRR.Code != http.StatusOK {
		t.Fatalf("commit status = %d, body=%s", commitRR.Code, commitRR.Body.String())
	}
	if !strings.Contains(commitRR.Body.String(), "patrons were imported") {
		t.Errorf("result page missing inserted summary text")
	}
	if !strings.Contains(commitRR.Body.String(), ">2<") {
		t.Errorf("result page missing the count 2 in <strong>")
	}

	patrons, _ := dm.GetAllPatrons()
	if len(patrons) != 2 {
		t.Errorf("expected 2 patrons, got %d", len(patrons))
	}
	for _, p := range patrons {
		if p.Username != "" {
			t.Errorf("records-only patron should have empty Username, got %q", p.Username)
		}
		if p.HasTempPassword {
			t.Errorf("records-only patron should have no temp password")
		}
	}
}

func TestImportCommit_WithLogins_CreatesUsersWithFlagAndTempSet(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	csv := "Name,Email\nAlice Brown,alice@example.com\n"
	req := multipartImportRequest(t, "/admin/patrons/import", csv, "with_logins", csrf)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	token := extractImportToken(t, rr.Body.String())

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("token", token)
	commitReq := httptest.NewRequest("POST", "/admin/patrons/import/confirm", strings.NewReader(form.Encode()))
	commitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	commitReq.AddCookie(sess)
	commitRR := httptest.NewRecorder()
	router.ServeHTTP(commitRR, commitReq)

	if commitRR.Code != http.StatusOK {
		t.Fatalf("commit status = %d", commitRR.Code)
	}
	if !strings.Contains(commitRR.Body.String(), "Credentials") {
		t.Errorf("expected Credentials section on result page")
	}

	user, err := dm.GetUserByUsername("abrown")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if !user.MustChangePassword {
		t.Errorf("imported user should have must_change_password=true")
	}
	if user.TempPassword == nil || *user.TempPassword == "" {
		t.Errorf("imported user should have non-empty temp_password")
	}
}

func TestImportCommit_TokenSingleUse(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	csv := "Name\nAlice\n"
	req := multipartImportRequest(t, "/admin/patrons/import", csv, "records_only", csrf)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	token := extractImportToken(t, rr.Body.String())

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("token", token)

	first := httptest.NewRecorder()
	commitReq := httptest.NewRequest("POST", "/admin/patrons/import/confirm", strings.NewReader(form.Encode()))
	commitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	commitReq.AddCookie(sess)
	router.ServeHTTP(first, commitReq)
	if first.Code != http.StatusOK {
		t.Fatalf("first commit status = %d", first.Code)
	}

	second := httptest.NewRecorder()
	commitReq2 := httptest.NewRequest("POST", "/admin/patrons/import/confirm", strings.NewReader(form.Encode()))
	commitReq2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	commitReq2.AddCookie(sess)
	router.ServeHTTP(second, commitReq2)
	if second.Code != http.StatusBadRequest {
		t.Errorf("second commit status = %d, want 400 (token already consumed)", second.Code)
	}
}

func TestImportDownload_SetsCacheNoStore(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	csv := "Name\nAlice\n"
	prevReq := multipartImportRequest(t, "/admin/patrons/import", csv, "with_logins", csrf)
	prevReq.AddCookie(sess)
	prevRR := httptest.NewRecorder()
	router.ServeHTTP(prevRR, prevReq)
	token := extractImportToken(t, prevRR.Body.String())

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("token", token)
	commitReq := httptest.NewRequest("POST", "/admin/patrons/import/confirm", strings.NewReader(form.Encode()))
	commitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	commitReq.AddCookie(sess)
	commitRR := httptest.NewRecorder()
	router.ServeHTTP(commitRR, commitReq)
	credToken := extractDownloadToken(t, commitRR.Body.String(), "credentials")
	if credToken == "" {
		t.Fatalf("credentials download token not found on result page")
	}

	dlReq := httptest.NewRequest("GET", "/admin/patrons/import/download/"+credToken, nil)
	dlReq.AddCookie(sess)
	dlRR := httptest.NewRecorder()
	router.ServeHTTP(dlRR, dlReq)

	if dlRR.Code != http.StatusOK {
		t.Fatalf("download status = %d", dlRR.Code)
	}
	if got := dlRR.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestImportDownload_SingleUse(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	csv := "Name\nAlice\n"
	prevReq := multipartImportRequest(t, "/admin/patrons/import", csv, "with_logins", csrf)
	prevReq.AddCookie(sess)
	prevRR := httptest.NewRecorder()
	router.ServeHTTP(prevRR, prevReq)
	token := extractImportToken(t, prevRR.Body.String())

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("token", token)
	commitReq := httptest.NewRequest("POST", "/admin/patrons/import/confirm", strings.NewReader(form.Encode()))
	commitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	commitReq.AddCookie(sess)
	commitRR := httptest.NewRecorder()
	router.ServeHTTP(commitRR, commitReq)
	credToken := extractDownloadToken(t, commitRR.Body.String(), "credentials")

	first := httptest.NewRecorder()
	dlReq := httptest.NewRequest("GET", "/admin/patrons/import/download/"+credToken, nil)
	dlReq.AddCookie(sess)
	router.ServeHTTP(first, dlReq)
	if first.Code != http.StatusOK {
		t.Fatalf("first download status = %d", first.Code)
	}

	second := httptest.NewRecorder()
	dlReq2 := httptest.NewRequest("GET", "/admin/patrons/import/download/"+credToken, nil)
	dlReq2.AddCookie(sess)
	router.ServeHTTP(second, dlReq2)
	if second.Code != http.StatusNotFound {
		t.Errorf("second download status = %d, want 404 (token consumed)", second.Code)
	}
}

func TestDefangCSVCell(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"normal":        "normal",
		"Alice Brown":   "Alice Brown",
		"=HYPERLINK(x)": "'=HYPERLINK(x)",
		"+CMD":          "'+CMD",
		"-2+1":          "'-2+1",
		"@SUM(A1)":      "'@SUM(A1)",
		"\tleading tab": "'\tleading tab",
		"\rleading cr":  "'\rleading cr",
		"a=safe":        "a=safe",
		"5 dollars":     "5 dollars",
	}
	for in, want := range cases {
		got := defangCSVCell(in)
		if got != want {
			t.Errorf("defangCSVCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestImportDownload_BodyDefangsFormulaCells(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, csrf := loginAs(t, dm, "admin", "admin")

	csv := "Name,Email\n=SneakyName,sneaky@example.com\n"
	prevReq := multipartImportRequest(t, "/admin/patrons/import", csv, "with_logins", csrf)
	prevReq.AddCookie(sess)
	prevRR := httptest.NewRecorder()
	router.ServeHTTP(prevRR, prevReq)
	token := extractImportToken(t, prevRR.Body.String())

	form := url.Values{}
	form.Set("csrf_token", csrf)
	form.Set("token", token)
	commitReq := httptest.NewRequest("POST", "/admin/patrons/import/confirm", strings.NewReader(form.Encode()))
	commitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	commitReq.AddCookie(sess)
	commitRR := httptest.NewRecorder()
	router.ServeHTTP(commitRR, commitReq)
	credToken := extractDownloadToken(t, commitRR.Body.String(), "credentials")
	if credToken == "" {
		t.Fatalf("expected credentials download even with sneaky name")
	}

	dlReq := httptest.NewRequest("GET", "/admin/patrons/import/download/"+credToken, nil)
	dlReq.AddCookie(sess)
	dlRR := httptest.NewRecorder()
	router.ServeHTTP(dlRR, dlReq)
	body := dlRR.Body.String()
	if strings.Contains(body, "\n=SneakyName") || strings.Contains(body, ",=SneakyName") {
		t.Errorf("downloaded CSV contains unescaped =SneakyName; defang not applied. body=%s", body)
	}
	if !strings.Contains(body, "'=SneakyName") {
		t.Errorf("expected '=SneakyName prefix after defang; got %s", body)
	}
}
