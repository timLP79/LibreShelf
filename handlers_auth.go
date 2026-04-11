package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var dummyPasswordHash []byte

func init() {
	hash, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to generate dummy bcrypt hash: %v", err)
	}
	dummyPasswordHash = hash
}

func generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func RequireAuth(c *gin.Context) {
	token, err := c.Cookie("session")
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	dm := getDB(c)
	user, err := dm.GetSession(token)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	c.Set("user", user)
	c.Next()
}

func RequireAdmin(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	if user.(*User).Role != "admin" {
		c.Status(http.StatusForbidden)
		renderTemplate(c, "error", gin.H{
			"Title":   "Forbidden",
			"Status":  403,
			"Message": "You don't have permission to access this page.",
		})
		c.Abort()
		return
	}

	c.Next()
}

func RequireStaff(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	if user.(*User).Role == "patron" {
		c.Status(http.StatusForbidden)
		renderTemplate(c, "error", gin.H{
			"Title":   "Forbidden",
			"Status":  403,
			"Message": "You don't have permission to access this page.",
		})
		c.Abort()
		return
	}

	c.Next()
}

func LoadUser(c *gin.Context) {
	token, err := c.Cookie("session")
	if err != nil {
		c.Next()
		return
	}

	dm := getDB(c)
	user, err := dm.GetSession(token)
	if err != nil {
		c.Next()
		return
	}

	c.Set("user", user)
	c.Next()
}

func HandleLogin(c *gin.Context) {
	c.Status(http.StatusOK)
	renderPage(c, "login", gin.H{
		"Title": "Login",
	})
}

func HandleLoginPost(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	dm := getDB(c)
	user, err := dm.GetUserByUsername(username)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("login lookup failed for %q: %v", username, err)
		renderPage(c, "login", gin.H{
			"Title": "Login",
			"Error": "Something went wrong.",
		})
		return
	}

	hashToCompare := dummyPasswordHash
	if user != nil {
		hashToCompare = []byte(user.PasswordHash)
	}
	bcryptErr := bcrypt.CompareHashAndPassword(hashToCompare, []byte(password))

	if user == nil || bcryptErr != nil {
		renderPage(c, "login", gin.H{
			"Title": "Login",
			"Error": "Invalid username or password.",
		})
		return
	}

	token, err := generateSessionToken()
	if err != nil {
		log.Printf("failed to generate session token: %v", err)
		renderPage(c, "login", gin.H{
			"Title": "Login",
			"Error": "Something went wrong.",
		})
		return
	}

	expiresAt := time.Now().Add(8 * time.Hour)
	if err := dm.CreateSession(token, user.ID, expiresAt); err != nil {
		log.Printf("Failed to create session: %v", err)
		renderPage(c, "login", gin.H{
			"Title": "Login",
			"Error": "Something went wrong.",
		})
		return
	}

	secure := os.Getenv("APP_ENV") == "production"
	c.SetCookie("session", token, 8*60*60, "/", "", secure, true)
	c.Redirect(http.StatusFound, "/")

}

func HandleLogout(c *gin.Context) {
	token, err := c.Cookie("session")
	if err == nil {
		dm := getDB(c)
		dm.DeleteSession(token)
	}

	c.SetCookie("session", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}
