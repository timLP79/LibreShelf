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

	renderTemplate(c, "patrons", gin.H{
		"Title":         "Patrons",
		"Patrons":       patrons,
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

	_, username, err := dm.CreatePatron(name, email, phone, string(hash))
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

	if name == "" {
		setFlash(c, flashKindError, "patron_name_required")
		c.Redirect(http.StatusFound, "/patrons")
		return
	}

	if err := dm.UpdatePatron(id, name, email, phone); err != nil {
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
