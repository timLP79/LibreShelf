// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func HandleChangePassword(c *gin.Context) {
	renderChangePasswordForm(c, "")
}

func HandleChangePasswordPost(c *gin.Context) {
	user := c.MustGet("user").(*User)

	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	if err := ValidatePassword(newPassword); err != nil {
		renderChangePasswordForm(c, err.Error())
		return
	}
	if newPassword != confirmPassword {
		renderChangePasswordForm(c, "Passwords do not match.")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("change password hash failed for user %d: %v", user.ID, err)
		renderChangePasswordForm(c, "Something went wrong.")
		return
	}

	if err := getDB(c).UpdateUserPassword(user.ID, string(hash)); err != nil {
		log.Printf("change password update failed for user %d: %v", user.ID, err)
		renderChangePasswordForm(c, "Something went wrong.")
		return
	}

	// UpdateUserPassword wipes all sessions; the cookie is now dead.
	secure := os.Getenv("APP_ENV") == "production"
	c.SetCookie("session", "", -1, "/", "", secure, true)
	c.Redirect(http.StatusFound, "/login")
}

func renderChangePasswordForm(c *gin.Context, errorMsg string) {
	user := c.MustGet("user").(*User)
	csrfToken, _ := c.Get("csrfToken")
	data := gin.H{
		"Title":     "Change Password",
		"User":      user,
		"CSRFToken": csrfToken,
		"Forced":    user.MustChangePassword,
	}
	if errorMsg != "" {
		data["Error"] = errorMsg
	}
	renderPage(c, "account_change_password", data)
}
