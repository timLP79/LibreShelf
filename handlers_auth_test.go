package main

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// authMiddlewareRouter spins up a minimal Gin router with just the
// requested middleware in front of a 200-OK handler. Used to exercise
// the "no user in context" path without going through RequireAuth,
// which would normally precede these.
func authMiddlewareRouter(t *testing.T, mw gin.HandlerFunc) *gin.Engine {
	t.Helper()
	if templates == nil {
		// renderTemplate (used by the 403 path) needs the error template parsed.
		templates = make(map[string]*template.Template)
	}
	if _, ok := templates["error"]; !ok {
		templates["error"] = template.Must(template.New("layout").ParseFiles(
			"templates/layout.html",
			"templates/error.html",
		))
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", mw, func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func TestRequireStaff_RedirectsWhenNoUser(t *testing.T) {
	r := authMiddlewareRouter(t, RequireStaff)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("GET", "/protected", nil))
	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want 302 redirect to /login", rr.Code)
	}
	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestRequireAdmin_RedirectsWhenNoUser(t *testing.T) {
	r := authMiddlewareRouter(t, RequireAdmin)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("GET", "/protected", nil))
	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want 302 redirect", rr.Code)
	}
}

func TestRequirePatron_RedirectsWhenNoUser(t *testing.T) {
	r := authMiddlewareRouter(t, RequirePatron)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("GET", "/protected", nil))
	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want 302 redirect", rr.Code)
	}
}

// TestRequireAuth_RedirectsOnExpiredSession exercises the GetSession
// failure branch -- valid cookie present but the session row no
// longer exists or is expired.
func TestRequireAuth_RedirectsOnExpiredSession(t *testing.T) {
	router, _ := setupTestRouter(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "this-token-does-not-exist"})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("expired-session request: status = %d, want 302", rr.Code)
	}
	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

// TestLoginSuccessSetsSessionCookie verifies the happy login path:
// POST with valid creds returns 302 to / and sets a session cookie.
// HandleLoginPost's success branch is the largest uncovered chunk
// (CreateUser + CreateSession + cookie write); this fills it.
func TestLoginSuccessSetsSessionCookie(t *testing.T) {
	router := setupAuthTestRouter(t)
	rr := postLogin(t, router, "admin", "Admin123!")

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body=%s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if loc != "/" {
		t.Errorf("Location = %q, want /", loc)
	}
	var sess *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "session" {
			sess = c
			break
		}
	}
	if sess == nil || sess.Value == "" {
		t.Errorf("expected session cookie set, got %v", rr.Result().Cookies())
	}
}
