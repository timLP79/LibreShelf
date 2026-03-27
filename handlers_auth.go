package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

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
	RequireAuth(c)
	if c.IsAborted() {
		return
	}

	user := c.MustGet("user").(*User)
	if user.Role != "admin" {
		c.Status(http.StatusForbidden)
		renderPage(c, "error", gin.H{
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
	if err == sql.ErrNoRows || user == nil {
		renderPage(c, "login", gin.H{
			"Title": "Login",
			"Error": "Invalid username or password.",
		})
		return
	}

	if err != nil {
		renderPage(c, "login", gin.H{
			"Title": "Login",
			"Error": "Something went wrong. Please try again.",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		renderPage(c, "login", gin.H{
			"Title": "Login",
			"Error": "Invalid username or password.",
		})
		return
	}

	token, err := generateSessionToken()
	if err != nil {
		renderPage(c, "login", gin.H{
			"Title": "Login",
			"Error": "Something went wrong. Please try again.",
		})
		return
	}

	expiresAt := time.Now().Add(8 * time.Hour)
	dm.CreateSession(token, user.ID, expiresAt)

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
