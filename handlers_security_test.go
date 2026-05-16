// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSecurityHeadersOnPublicPage pins the SecurityHeaders middleware
// on the /login page (a non-authenticated route, so no extra setup
// needed). Failure here means a future edit dropped the middleware
// from the router.
func TestSecurityHeadersOnPublicPage(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/login", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
	}
	for name, want := range checks {
		if got := rr.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatalf("Content-Security-Policy header missing")
	}
	for _, fragment := range []string{
		"default-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"img-src 'self' data:",
		"https://books.google.com",
	} {
		if !strings.Contains(csp, fragment) {
			t.Errorf("CSP missing %q; got %q", fragment, csp)
		}
	}
}

// TestSecurityHeadersOn404 pins that even error responses carry the
// security headers, since the middleware runs at the router level
// before route matching.
func TestSecurityHeadersOn404(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/this/does/not/exist", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200 for unknown route, got %d", rr.Code)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options on error response = %q, want nosniff", got)
	}
}

// TestHSTSGatedOnAppEnv verifies HSTS is OFF by default (bare-IP HTTP
// deploys must not advertise HSTS) and ON when APP_ENV=production.
func TestHSTSGatedOnAppEnv(t *testing.T) {
	router, _ := setupTestRouter(t)

	t.Run("default off", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
			t.Errorf("HSTS should be empty in default env, got %q", got)
		}
	})

	t.Run("production on", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		req := httptest.NewRequest("GET", "/login", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		got := rr.Header().Get("Strict-Transport-Security")
		if !strings.Contains(got, "max-age=") {
			t.Errorf("HSTS in production should contain max-age=, got %q", got)
		}
	})
}
