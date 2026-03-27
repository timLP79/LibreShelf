package main

import (
	"html/template"
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

	// Load Templates
	templates = make(map[string]*template.Template)
	templateNames := []string{
		"index", "catalog", "book_detail",
		"patrons", "admin", "kiosk",
	}
	for _, name := range templateNames {
		templates[name] = template.Must(template.ParseFiles(
			"templates/layout.html",
			"templates/"+name+".html",
		))
	}

	templates["login"] = template.Must(template.ParseFiles(
		"templates/login.html",
	))

	templates["error"] = template.Must(template.ParseFiles(
		"templates/layout.html",
		"templates/error.html",
	))

	// Setup router
	router := gin.Default()

	// Static files
	router.Static("/stylesheets", "static/stylesheets")
	router.Static("/javascripts", "static/javascripts")
	router.Static("/images", "static/images")

	// Database middleware - make dm available to all handlers
	router.Use(DatabaseMiddleware(dm))

	// Public routes
	router.GET("/login", HandleLogin)
	router.POST("/login", HandleLoginPost)
	router.POST("/logout", HandleLogout)
	router.GET("/kiosk", HandleKiosk)

	// Authenticated routes
	auth := router.Group("/")
	auth.Use(RequireAuth)
	auth.GET("/", HandleIndex)
	auth.GET("/catalog", HandleCatalog)
	auth.GET("/books/:id", HandleBookDetail)

	// Admin-only routes
	admin := router.Group("/")
	admin.Use(RequireAdmin)
	admin.GET("/patrons", HandlePatrons)
	admin.GET("/admin", HandleAdmin)

	router.NoRoute(HandleNotFound)

	// Start server
	router.Run(":" + port)
}
