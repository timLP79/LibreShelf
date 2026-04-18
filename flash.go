package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

const (
	flashMaxAgeSeconds = 60
	flashKindError     = "error"
	flashKindSuccess   = "success"
)

// flashMessages maps a stable code slug to the human-visible banner text.
// The cookie only carries the slug; the map lives server-side so that
// operator-visible strings never transit the cookie jar, access logs, or
// request referers. Add a new entry here whenever a handler emits a new
// code.
var flashMessages = map[string]string{
	"password_mismatch":        "Password and confirmation did not match.",
	"weak_password":            "Password does not meet complexity requirements.",
	"invalid_username":         "Username may only contain letters, numbers, and underscores.",
	"invalid_role":             "Role must be either 'admin' or 'staff'.",
	"duplicate_username":       "That username is already taken.",
	"staff_created":            "Staff account created.",
	"cannot_demote_self":       "You cannot demote your own account.",
	"cannot_demote_last_admin": "At least one admin account must remain.",
	"staff_updated":            "Account updated.",
	"cannot_delete_self":       "You cannot delete your own account.",
	"cannot_delete_last_admin": "At least one admin account must remain.",
	"staff_deleted":            "Account deleted.",
	"password_reset":           "Password reset. Share the new password with the user through a trusted channel.",
}

func flashCookieName(kind string) string {
	switch kind {
	case flashKindError:
		return "flash_error"
	case flashKindSuccess:
		return "flash_success"
	}
	return ""
}

// setFlash writes a short-lived, HttpOnly, SameSite=Strict cookie
// carrying an error-code slug. The paired readAndClearFlash on the next
// request resolves the slug to a message and clears the cookie so a
// browser refresh does not re-show the banner.
func setFlash(c *gin.Context, kind, code string) {
	name := flashCookieName(kind)
	if name == "" {
		return
	}
	secure := os.Getenv("APP_ENV") == "production"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(name, code, flashMaxAgeSeconds, "/", "", secure, true)
}

// readAndClearFlash returns the banner message for the kind's flash
// cookie, or "" when no cookie is present. It always clears the cookie
// (MaxAge=-1) in the same response. An unknown code is logged and
// returns "" so a typo surfaces in server logs rather than silently
// showing a blank banner in production.
func readAndClearFlash(c *gin.Context, kind string) string {
	name := flashCookieName(kind)
	if name == "" {
		return ""
	}
	code, err := c.Cookie(name)
	if err != nil || code == "" {
		return ""
	}
	secure := os.Getenv("APP_ENV") == "production"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(name, "", -1, "/", "", secure, true)
	msg, ok := flashMessages[code]
	if !ok {
		log.Printf("flash: unknown code %q for kind %q", code, kind)
		return ""
	}
	return msg
}
