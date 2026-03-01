package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DatabaseMiddleware store the DatabaseManager in the Gin context
func DatabaseMiddleware(dm *DatabaseManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("db", dm)
		c.Next()
	}
}

// getDB retrieves the DatabaseManager from the Gin context
func getDB(c *gin.Context) *DatabaseManager {
	return c.MustGet("db").(*DatabaseManager)
}

// HandleIndex renders Dashboard
func HandleIndex(c *gin.Context) {
	renderTemplate(c, "index", gin.H{
		"Title": "Dashboard",
	})
}

// HandleCatalog renders the Catalog page
func HandleCatalog(c *gin.Context) {
	renderTemplate(c, "catalog", gin.H{
		"Title": "Catalog",
	})
}

// HandleBookDetail renders the book detail page
func HandleBookDetail(c *gin.Context) {
	id := c.Param("id")
	renderTemplate(c, "book_detail", gin.H{
		"Title": "Book #" + id,
	})
}

// HandlePatrons renders the Patrons page
func HandlePatrons(c *gin.Context) {
	renderTemplate(c, "patrons", gin.H{
		"Title": "Patrons",
	})
}

// HandleAdmin renders the Admin page
func HandleAdmin(c *gin.Context) {
	renderTemplate(c, "admin", gin.H{
		"Title": "Admin",
	})
}

// HandleKiosk renders the Kiosk page
func HandleKiosk(c *gin.Context) {
	renderTemplate(c, "kiosk", gin.H{
		"Title": "Kiosk",
	})
}

// HandleNotFound renders the 404 error page
func HandleNotFound(c *gin.Context) {
	c.Status(http.StatusNotFound)
	renderTemplate(c, "error", gin.H{
		"Title":   "Not Found",
		"Status":  404,
		"Message": "Page not found",
	})
}

// renderTemplate is a helper to execute a name template
func renderTemplate(c *gin.Context, name string, data gin.H) {
	tmpl, ok := templates[name]
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}
	tmpl.ExecuteTemplate(c.Writer, "layout", data)
}
