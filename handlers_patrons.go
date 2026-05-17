// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func HandlePatronList(c *gin.Context) {
	dm := getDB(c)
	patrons, err := dm.GetAllPatrons()
	if err != nil {
		log.Printf("HandlePatronList: GetAllPatrons: %v", err)
		c.Status(http.StatusInternalServerError)
		renderTemplate(c, "error", gin.H{
			"Title":   "Error",
			"Status":  500,
			"Message": "Unable to load patrons.",
		})
		return
	}

	user := c.MustGet("user").(*User)
	canImport := user.Role == "admin" ||
		(user.Role == "staff" && dm.GetSettingBool("staff_can_import_patrons", false))

	renderTemplate(c, "patrons", gin.H{
		"Title":         "Patrons",
		"Patrons":       patrons,
		"CanImport":     canImport,
		"Success":       readAndClearFlash(c, flashKindSuccess),
		"SuccessDetail": readAndClearFlashDetail(c),
		"Error":         readAndClearFlash(c, flashKindError),
	})
}

func HandlePatronCreate(c *gin.Context) {
	dm := getDB(c)

	name := normalizeFreeText(c.PostForm("name"))
	email := strings.TrimSpace(c.PostForm("email"))
	phone := strings.TrimSpace(c.PostForm("phone"))
	address := strings.TrimSpace(c.PostForm("address"))
	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password_confirm")

	if name == "" {
		setFlash(c, flashKindError, "patron_name_required")
		c.Redirect(http.StatusFound, "/patrons")
		return
	}

	// If the name contains no alphanumerics, generateBaseUsername will
	// return "" and the DB call would fail anyway. Catch it up front so
	// the admin sees a meaningful banner instead of a 500.
	if generateBaseUsername(name) == "" {
		setFlash(c, flashKindError, "patron_name_unusable")
		c.Redirect(http.StatusFound, "/patrons")
		return
	}

	if password != passwordConfirm {
		setFlash(c, flashKindError, "password_mismatch")
		c.Redirect(http.StatusFound, "/patrons")
		return
	}

	if err := ValidatePassword(password); err != nil {
		setFlash(c, flashKindError, "weak_password")
		c.Redirect(http.StatusFound, "/patrons")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("HandlePatronCreate: bcrypt: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	_, username, err := dm.CreatePatron(name, email, phone, address, string(hash))
	if err != nil {
		log.Printf("HandlePatronCreate: CreatePatron: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	setFlash(c, flashKindSuccess, "patron_created")
	// Detail shows both name and the generated username so the admin
	// has the credential to share without a second round-trip to the
	// list page.
	setFlashDetail(c, name+" ("+username+")")
	c.Redirect(http.StatusFound, "/patrons")
}

func HandlePatronEdit(c *gin.Context) {
	dm := getDB(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleNotFound(c)
		return
	}

	// 404 if the patron does not exist. :id is a patron_id (not a
	// user_id), so there is no role-crossing IDOR to guard against --
	// every row in patrons IS a patron by construction. This is a
	// plain not-found check.
	if _, err := dm.GetPatronByID(id); err == sql.ErrNoRows {
		HandleNotFound(c)
		return
	} else if err != nil {
		log.Printf("HandlePatronEdit: GetPatronByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	name := normalizeFreeText(c.PostForm("name"))
	email := strings.TrimSpace(c.PostForm("email"))
	phone := strings.TrimSpace(c.PostForm("phone"))
	address := strings.TrimSpace(c.PostForm("address"))

	if name == "" {
		setFlash(c, flashKindError, "patron_name_required")
		c.Redirect(http.StatusFound, "/patrons")
		return
	}

	if err := dm.UpdatePatron(id, name, email, phone, address); err != nil {
		log.Printf("HandlePatronEdit: UpdatePatron: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	setFlash(c, flashKindSuccess, "patron_updated")
	setFlashDetail(c, name)
	c.Redirect(http.StatusFound, "/patrons")
}

func HandlePatronDelete(c *gin.Context) {
	dm := getDB(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleNotFound(c)
		return
	}

	patron, err := dm.GetPatronByID(id)
	if err == sql.ErrNoRows {
		HandleNotFound(c)
		return
	}
	if err != nil {
		log.Printf("HandlePatronDelete: GetPatronByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	if err := dm.DeletePatron(id); err != nil {
		if errors.Is(err, ErrPatronHasLoans) {
			setFlash(c, flashKindError, "patron_has_loans")
			c.Redirect(http.StatusFound, "/patrons")
			return
		}
		log.Printf("HandlePatronDelete: DeletePatron: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	setFlash(c, flashKindSuccess, "patron_deleted")
	setFlashDetail(c, patron.Name)
	c.Redirect(http.StatusFound, "/patrons")
}

func HandlePatronLoginCredentials(c *gin.Context) {
	dm := getDB(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleNotFound(c)
		return
	}
	patron, err := dm.GetPatronByID(id)
	if err == sql.ErrNoRows {
		HandleNotFound(c)
		return
	}
	if err != nil {
		log.Printf("HandlePatronLoginCredentials: GetPatronByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if patron.Username == "" {
		setFlash(c, flashKindError, "temp_password_unavailable")
		c.Redirect(http.StatusFound, "/patrons")
		return
	}
	user, err := dm.GetUserByUsername(patron.Username)
	if err != nil {
		log.Printf("HandlePatronLoginCredentials: GetUserByUsername: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if user.TempPassword == nil {
		setFlash(c, flashKindError, "temp_password_unavailable")
		c.Redirect(http.StatusFound, "/patrons")
		return
	}

	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("Pragma", "no-cache")

	renderTemplate(c, "patron_login_credentials", gin.H{
		"Title":        "Login Setup",
		"Patron":       patron,
		"TempPassword": *user.TempPassword,
		"Success":      readAndClearFlash(c, flashKindSuccess),
	})
}

func HandlePatronDismissTemp(c *gin.Context) {
	dm := getDB(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleNotFound(c)
		return
	}
	patron, err := dm.GetPatronByID(id)
	if err == sql.ErrNoRows {
		HandleNotFound(c)
		return
	}
	if err != nil {
		log.Printf("HandlePatronDismissTemp: GetPatronByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if patron.Username == "" {
		c.Redirect(http.StatusSeeOther, "/patrons")
		return
	}
	user, err := dm.GetUserByUsername(patron.Username)
	if err != nil {
		log.Printf("HandlePatronDismissTemp: GetUserByUsername: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if err := dm.ClearTempPassword(user.ID); err != nil {
		log.Printf("HandlePatronDismissTemp: ClearTempPassword: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	setFlash(c, flashKindSuccess, "temp_password_dismissed")
	setFlashDetail(c, patron.Name)
	c.Redirect(http.StatusSeeOther, "/patrons")
}

func HandlePatronRegenerateTemp(c *gin.Context) {
	dm := getDB(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleNotFound(c)
		return
	}
	patron, err := dm.GetPatronByID(id)
	if err == sql.ErrNoRows {
		HandleNotFound(c)
		return
	}
	if err != nil {
		log.Printf("HandlePatronRegenerateTemp: GetPatronByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if patron.Username == "" {
		setFlash(c, flashKindError, "temp_password_unavailable")
		c.Redirect(http.StatusSeeOther, "/patrons")
		return
	}
	user, err := dm.GetUserByUsername(patron.Username)
	if err != nil {
		log.Printf("HandlePatronRegenerateTemp: GetUserByUsername: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if _, err := dm.RegenerateTempPassword(user.ID); err != nil {
		log.Printf("HandlePatronRegenerateTemp: RegenerateTempPassword: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	setFlash(c, flashKindSuccess, "temp_password_regenerated")
	c.Redirect(http.StatusSeeOther, "/patrons/"+strconv.Itoa(id)+"/login-credentials")
}
