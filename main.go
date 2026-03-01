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

	// Load Templates
	templates = make(map[string]*template.Template)
	templateNames := []string{
		"index", "catalog", "book_detail",
		"patrons", "admin", "kiosk", "error",
	}
	for _, name := range templateNames {
		templates[name] = template.Must(template.ParseFiles(
			"templates/layout.html",
			"templates/"+name+".html",
		))
	}

	// Setup router
	router := gin.Default()

	// Static files
	router.Static("/stylesheets", "static/stylesheets")
	router.Static("/javascripts", "static/javascripts")
	router.Static("/images", "static/images")

	// Database middleware - make dm available to all handlers
	router.Use(DatabaseMiddleware(dm))

	// Routes
	router.GET("/", HandleIndex)
	router.GET("/catalog", HandleCatalog)
	router.GET("/books/:id", HandleBookDetail)
	router.GET("/patrons", HandlePatrons)
	router.GET("/admin", HandleAdmin)
	router.GET("/kiosk", HandleKiosk)
	router.NoRoute(HandleNotFound)

	// Start server
	router.Run(":" + port)
}
