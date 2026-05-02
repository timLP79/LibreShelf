package main

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupExport_Happy(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "admin", "admin")

	// Drop one real cover and one .bak in the covers dir under DATA_DIR.
	coversPath := filepath.Join(os.Getenv("DATA_DIR"), "covers")
	if err := os.MkdirAll(coversPath, 0o755); err != nil {
		t.Fatalf("mkdir covers: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coversPath, "real.jpg"), []byte("\xff\xd8\xff\xe0fake"), 0o644); err != nil {
		t.Fatalf("write real cover: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coversPath, "stale.jpg.bak"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write bak: %v", err)
	}

	req := httptest.NewRequest("GET", "/admin/backup/export", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	cd := rr.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "libreshelf-backup-") || !strings.Contains(cd, ".zip") {
		t.Errorf("Content-Disposition = %q, expected libreshelf-backup-*.zip", cd)
	}

	body := rr.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	var foundDB, foundReal bool
	for _, f := range zr.File {
		switch {
		case f.Name == "database.sqlite":
			foundDB = true
			if f.UncompressedSize64 == 0 {
				t.Errorf("database.sqlite is empty")
			}
		case f.Name == "covers/real.jpg":
			foundReal = true
		case strings.HasSuffix(f.Name, ".bak"):
			t.Errorf("backup ZIP contains a .bak file: %q", f.Name)
		}
	}
	if !foundDB {
		t.Errorf("backup ZIP missing database.sqlite")
	}
	if !foundReal {
		t.Errorf("backup ZIP missing covers/real.jpg")
	}
}

func TestBackupExport_RejectsNonAdmin(t *testing.T) {
	router, dm := setupTestRouter(t)
	sess, _ := loginAs(t, dm, "staffuser", "staff")

	req := httptest.NewRequest("GET", "/admin/backup/export", nil)
	req.AddCookie(sess)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Errorf("staff user should not be able to export backup; got 200")
	}
}
