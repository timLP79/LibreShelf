package main

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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
