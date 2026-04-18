package main

import (
	"html/template"
	"log"
	"os"

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
		"patrons", "admin", "kiosk", "staff",
	}
	for _, name := range templateNames {
		templates[name] = template.Must(template.New("layout").Funcs(funcMap).ParseFiles(
			"templates/layout.html",
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

	// Authenticated routes -- any logged in user
	auth := router.Group("/")
	auth.Use(RequireAuth, CSRFProtect)
	auth.GET("/", HandleIndex)
	auth.GET("/catalog", HandleCatalog)
	auth.GET("/books/:id", HandleBookDetail)
	auth.POST("/logout", HandleLogout)

	// Staff routes -- admin + staff
	staff := router.Group("/")
	staff.Use(RequireAuth, RequireStaff, CSRFProtect)
	staff.GET("/patrons", HandlePatrons)
	staff.GET("/admin", HandleAdmin)
	staff.GET("/api/openlibrary/isbn/:isbn", HandleOpenLibraryLookup)
	staff.GET("/books/new", HandleBookNew)
	staff.POST("/books", HandleBookCreate)
	staff.GET("/books/:id/edit", HandleBookEdit)
	staff.POST("/books/:id/edit", HandleBookUpdate)

	// Admin-only routes
	admin := router.Group("/")
	admin.Use(RequireAuth, RequireAdmin, CSRFProtect)
	admin.GET("/staff", HandleStaffList)
	admin.POST("/staff", HandleStaffCreate)
	admin.POST("/staff/:id/edit", HandleStaffEdit)
	admin.POST("/staff/:id/delete", HandleStaffDelete)
	admin.POST("/staff/:id/password", HandleStaffResetPassword)
	admin.POST("/books/:id/delete", HandleBookDelete)

	router.NoRoute(HandleNotFound)

	// Start server
	router.Run(":" + port)
}
