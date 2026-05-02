package main

import (
	"context"
	"html/template"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

var templates map[string]*template.Template

func main() {
	// Configuration
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "database.sqlite"
	}

	// Initialize the database
	dm := NewDatabaseManager(dataDir + "/" + dbName)
	dm.SeedDefaultUsers()
	dm.SeedBooks()

	// Opportunistically backfill covers from Open Library for any book
	// that has an ISBN but no cover file yet. Safe to call every
	// startup: the inner SELECT is a no-op after all seed books have
	// their covers. 60s total budget so a slow OL (or network block)
	// cannot wedge the server at boot -- the inner HTTP client also
	// has its own 10s per-request timeout.
	seedCoverCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	dm.FetchAndStoreSeedCovers(seedCoverCtx)
	cancel()

	// Template helpers
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
		"index", "catalog", "book_detail", "book_form",
		"patrons", "admin", "staff", "loans", "my_loans",
		"backup_admin",
	}
	for _, name := range templateNames {
		templates[name] = template.Must(template.New("layout").Funcs(funcMap).ParseFiles(
			"templates/layout.html",
			"templates/"+name+".html",
		))
	}

	kioskTemplateNames := []string{"kiosk", "kiosk_book_detail"}
	for _, name := range kioskTemplateNames {
		templates[name] = template.Must(template.New("kiosk_layout").Funcs(funcMap).ParseFiles(
			"templates/kiosk_layout.html",
			"templates/"+name+".html",
		))
	}

	templates["login"] = template.Must(template.ParseFiles(
		"templates/login.html",
	))

	templates["error"] = template.Must(template.New("layout").Funcs(funcMap).ParseFiles(
		"templates/layout.html",
		"templates/error.html",
	))

	// Setup router
	router := gin.Default()

	// Static files
	router.Static("/stylesheets", "static/stylesheets")
	router.Static("/javascripts", "static/javascripts")
	router.Static("/images", "static/images")
	if err := os.MkdirAll(coversDir(), 0o755); err != nil {
		log.Fatalf("failed to create covers dir: %v", err)
	}
	router.Static("/covers", coversDir())

	// Database middleware - make dm available to all handlers
	router.Use(DatabaseMiddleware(dm))

	// Public routes
	router.GET("/login", HandleLogin)
	router.POST("/login", LoginCSRFProtect, HandleLoginPost)
	router.GET("/kiosk", HandleKiosk)
	router.GET("/kiosk/books/:id", HandleKioskBookDetail)

	// Authenticated routes -- any logged in user
	auth := router.Group("/")
	auth.Use(RequireAuth, CSRFProtect)
	auth.GET("/", HandleIndex)
	auth.GET("/catalog", HandleCatalog)
	auth.GET("/books/:id", HandleBookDetail)
	auth.POST("/logout", HandleLogout)

	// Patron-only routes
	patron := router.Group("/")
	patron.Use(RequireAuth, RequirePatron, CSRFProtect)
	patron.GET("/my/loans", HandleMyLoans)

	// Staff routes -- admin + staff
	staff := router.Group("/")
	staff.Use(RequireAuth, RequireStaff, CSRFProtect)
	staff.GET("/patrons", HandlePatronList)
	staff.POST("/patrons", HandlePatronCreate)
	staff.POST("/patrons/:id/edit", HandlePatronEdit)
	staff.POST("/patrons/:id/delete", HandlePatronDelete)
	staff.GET("/admin", HandleAdmin)
	staff.GET("/api/openlibrary/isbn/:isbn", HandleOpenLibraryLookup)
	staff.GET("/books/new", HandleBookNew)
	staff.POST("/books", HandleBookCreate)
	staff.GET("/books/:id/edit", HandleBookEdit)
	staff.POST("/books/:id/edit", HandleBookUpdate)
	staff.POST("/books/:id/checkout", HandleCheckout)
	staff.POST("/loans/:id/return", HandleReturn)
	staff.GET("/loans", HandleLoansList)

	// Admin-only routes
	admin := router.Group("/")
	admin.Use(RequireAuth, RequireAdmin, CSRFProtect)
	admin.GET("/staff", HandleStaffList)
	admin.POST("/staff", HandleStaffCreate)
	admin.POST("/staff/:id/edit", HandleStaffEdit)
	admin.POST("/staff/:id/delete", HandleStaffDelete)
	admin.POST("/staff/:id/password", HandleStaffResetPassword)
	admin.POST("/books/:id/delete", HandleBookDelete)
	admin.GET("/admin/backup", HandleBackupAdmin)
	admin.GET("/admin/backup/export", HandleBackupExport)

	router.NoRoute(HandleNotFound)

	// Start server
	router.Run(":" + port)
}
