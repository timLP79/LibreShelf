package main

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	// Create a temp database
	tmpDir, err := os.MkdirTemp("", "librashelf-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dm := NewDatabaseManager(tmpDir + "/test.sqlite")
	dm.SeedBooks()

	funcMap := template.FuncMap{
		"deref": func(v interface{}) interface{} {
			switch p := v.(type) {
			case *string:
				if p != nil {
					return *p
				}
			case *int:
				if p != nil {
					return *p
				}
			}
			return ""
		},
	}

	templates = make(map[string]*template.Template)
	templateNames := []string{
		"index", "catalog", "book_detail",
		"patrons", "admin", "kiosk", "error",
	}

	for _, name := range templateNames {
		templates[name] = template.Must(template.New("layout").Funcs(funcMap).ParseFiles(
			"templates/layout.html",
			"templates/"+name+".html",
		))
	}

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(DatabaseMiddleware(dm))
	router.GET("/", HandleIndex)
	router.GET("/catalog", HandleCatalog)
	router.GET("/books/:id", HandleBookDetail)
	router.GET("/patrons", HandlePatrons)
	router.GET("/admin", HandleAdmin)
	router.GET("/kiosk", HandleKiosk)
	router.NoRoute(HandleNotFound)

	return router
}

func TestIndexRoute(t *testing.T) {
	router := setupTestRouter(t)

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Dashboard") {
		t.Errorf("Expected body to contain 'Dashboard'")
	}
	t.Log("Index route test passed!")
}

func TestAllRoutesReturn200(t *testing.T) {
	router := setupTestRouter(t)

	routes := []string{"/", "/catalog", "/patrons", "/admin", "/kiosk", "/books/1"}
	for _, route := range routes {
		req, _ := http.NewRequest("GET", route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Route %s: expected 200, got %d", route, rr.Code)
		}
	}
}

func TestNotFoundReturns404(t *testing.T) {
	router := setupTestRouter(t)

	req, _ := http.NewRequest("GET", "/doesnotexist", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestBookDetailNotFoundReturns404(t *testing.T) {
	router := setupTestRouter(t)

	req, _ := http.NewRequest("GET", "/books/9999", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestBookDetailNonNumericReturns404(t *testing.T) {
	router := setupTestRouter(t)

	req, _ := http.NewRequest("GET", "/books/abc", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

// TestResponseContentTypeIsHTML verifies that the renderTemplate helper
// sets Content-Type explicitly (#31). Previously we relied on Go's body
// sniffing, which worked accidentally; the buffer-based rewrite sets it
// explicitly and this test pins that behavior.
func TestResponseContentTypeIsHTML(t *testing.T) {
	router := setupTestRouter(t)

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got %q", ct)
	}
}

// setupAuthTestRouter builds a router with only the auth routes and the
// templates they need (login, error). It seeds the default users so that
// realistic login attempts can be made against real bcrypt hashes.
func setupAuthTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "libreshelf-auth-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dm := NewDatabaseManager(tmpDir + "/test.sqlite")
	dm.SeedDefaultUsers()

	funcMap := template.FuncMap{
		"deref": func(v interface{}) interface{} {
			switch p := v.(type) {
			case *string:
				if p != nil {
					return *p
				}
			case *int:
				if p != nil {
					return *p
				}
			}
			return ""
		},
	}

	templates = make(map[string]*template.Template)
	templates["login"] = template.Must(template.ParseFiles("templates/login.html"))
	templates["error"] = template.Must(template.New("layout").Funcs(funcMap).ParseFiles(
		"templates/layout.html",
		"templates/error.html",
	))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(DatabaseMiddleware(dm))
	router.POST("/login", HandleLoginPost)
	return router
}

func postLogin(t *testing.T, router *gin.Engine, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("username", username)
	form.Set("password", password)
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// TestLoginErrorIsGeneric asserts that both "user does not exist" and
// "user exists but password is wrong" render the same generic error
// message in the response body. Prevents regressions where a future
// edit accidentally leaks which branch was taken.
func TestLoginErrorIsGeneric(t *testing.T) {
	router := setupAuthTestRouter(t)

	const expected = "Invalid username or password"

	fakeRR := postLogin(t, router, "does-not-exist", "irrelevant")
	if !strings.Contains(fakeRR.Body.String(), expected) {
		t.Errorf("fake-user response missing %q in body", expected)
	}

	realRR := postLogin(t, router, "staff1", "wrong-password")
	if !strings.Contains(realRR.Body.String(), expected) {
		t.Errorf("wrong-password response missing %q in body", expected)
	}
}

// TestLoginTimingIsConstant asserts that login requests for nonexistent
// users take roughly the same wall-clock time as login requests for
// existing users with a wrong password. If the handler skips bcrypt when
// the username is missing, the fake-user path will be ~1ms while the
// real-user path will be ~60ms (default bcrypt cost), leaking username
// existence via timing (#33).
func TestLoginTimingIsConstant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test in short mode")
	}

	router := setupAuthTestRouter(t)

	measure := func(username, password string) time.Duration {
		start := time.Now()
		postLogin(t, router, username, password)
		return time.Since(start)
	}

	// Warm up once per path to avoid first-call overhead skewing the first sample.
	measure("warmup-nobody", "x")
	measure("staff1", "x")

	const samples = 15
	fakeDurations := make([]time.Duration, samples)
	realDurations := make([]time.Duration, samples)
	for i := 0; i < samples; i++ {
		fakeDurations[i] = measure("does-not-exist", "irrelevant")
		realDurations[i] = measure("staff1", "wrong-password")
	}

	fakeMedian := medianDuration(fakeDurations)
	realMedian := medianDuration(realDurations)

	t.Logf("fake-user median: %v", fakeMedian)
	t.Logf("real-user median: %v", realMedian)

	// Fail if the fake-user path is less than half the real-user path.
	// With the bug present: fake ~1ms, real ~60ms, ratio ~0.017 -> fail.
	// With the fix: both ~60ms, ratio ~1.0 -> pass.
	if fakeMedian*2 < realMedian {
		t.Errorf("login timing leaks username existence: fake=%v real=%v (fake should be at least half of real)",
			fakeMedian, realMedian)
	}
}

func medianDuration(ds []time.Duration) time.Duration {
	sorted := make([]time.Duration, len(ds))
	copy(sorted, ds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}
