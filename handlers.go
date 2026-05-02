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

// DBReadLock holds the DatabaseManager's read lock for the duration of
// the request. Allows many concurrent readers but blocks while
// HandleBackupImport holds the write lock during a database swap.
// Routes that need exclusive DB access (only the import endpoint) must
// be in a group that does NOT include this middleware, and must take
// dm.mu.Lock() themselves.
func DBReadLock(c *gin.Context) {
	dm := getDB(c)
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	c.Next()
}

// HandleIndex renders Dashboard
func HandleIndex(c *gin.Context) {
	dm := getDB(c)
	user, _ := c.Get("user")
	u := user.(*User)

	data := gin.H{"Title": "Dashboard"}

	if u.Role == "patron" {
		if u.PatronID == nil {
			log.Printf("HandleIndex: patron user %d has nil patron_id", u.ID)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		loans, err := dm.GetPatronActiveLoans(*u.PatronID)
		if err != nil {
			log.Printf("HandleIndex: GetPatronActiveLoans(%d): %v", *u.PatronID, err)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		data["MyLoanCount"] = len(loans)
		if len(loans) > 0 {
			data["NextDueDate"] = loans[0].DueDate
		}
	} else {
		active, err := dm.CountActiveLoans()
		if err != nil {
			log.Printf("HandleIndex: CountActiveLoans: %v", err)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		overdue, err := dm.CountOverdueLoans()
		if err != nil {
			log.Printf("HandleIndex: CountOverdueLoans: %v", err)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		oos, err := dm.CountOutOfStock()
		if err != nil {
			log.Printf("HandleIndex: CountOutOfStock: %v", err)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		data["ActiveLoans"] = active
		data["OverdueLoans"] = overdue
		data["OutOfStock"] = oos
	}

	renderTemplate(c, "index", data)
}

// HandleAdmin renders the Admin page
func HandleAdmin(c *gin.Context) {
	renderTemplate(c, "admin", gin.H{
		"Title": "Admin",
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

func renderKioskTemplate(c *gin.Context, name string, data gin.H) {
	tmpl, ok := templates[name]
	if !ok {
		log.Printf("template not found: %q", name)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "kiosk_layout", data); err != nil {
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
