package main

import (
	"bytes"
	"log"
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

func renderTemplate(c *gin.Context, name string, data gin.H) {
	tmpl, ok := templates[name]
	if !ok {
		log.Printf("template not found: %q", name)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if user, exists := c.Get("user"); exists {
		data["User"] = user
	}
	if token, exists := c.Get("csrfToken"); exists {
		data["CSRFToken"] = token
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		log.Printf("template execution failed for %q: %v", name, err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := c.Writer.Write(buf.Bytes()); err != nil {
		log.Printf("response write failed for %q: %v", name, err)
	}
}

func renderPage(c *gin.Context, name string, data gin.H) {
	tmpl, ok := templates[name]
	if !ok {
		log.Printf("template not found: %q", name)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("template execution failed for %q: %v", name, err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := c.Writer.Write(buf.Bytes()); err != nil {
		log.Printf("response write failed for %q: %v", name, err)
	}
}
