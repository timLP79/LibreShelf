package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func HandleStaffList(c *gin.Context) {
	dm := getDB(c)
	user := c.MustGet("user").(*User)

	staff, err := dm.GetAllStaff()
	if err != nil {
		log.Printf("HandleStaffList: GetAllStaff: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	adminCount, err := dm.CountAdmins()
	if err != nil {
		log.Printf("HandleStaffList: CountAdmins: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	renderTemplate(c, "staff", gin.H{
		"Title":         "Staff Management",
		"Staff":         staff,
		"CurrentUserID": user.ID,
		"AdminCount":    adminCount,
		"Error":         readAndClearFlash(c, flashKindError),
		"Success":       readAndClearFlash(c, flashKindSuccess),
	})
}

func HandleStaffCreate(c *gin.Context) {
	dm := getDB(c)
	username := c.PostForm("username")
	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password_confirm")
	role := c.PostForm("role")

	if password != passwordConfirm {
		setFlash(c, flashKindError, "password_mismatch")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	if err := ValidatePassword(password); err != nil {
		setFlash(c, flashKindError, "weak_password")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	if err := ValidateUsername(username); err != nil {
		setFlash(c, flashKindError, "invalid_username")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	if role != "admin" && role != "staff" {
		setFlash(c, flashKindError, "invalid_role")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	_, err := dm.GetUserByUsername(username)
	if err == nil {
		setFlash(c, flashKindError, "duplicate_username")
		c.Redirect(http.StatusFound, "/staff")
		return
	}
	if err != sql.ErrNoRows {
		log.Printf("HandleStaffCreate: GetUserByUsername: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("HandleStaffCreate: bcrypt: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	if err := dm.CreateUser(username, string(hash), role, nil); err != nil {
		log.Printf("HandleStaffCreate: CreateUser: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	setFlash(c, flashKindSuccess, "staff_created")
	c.Redirect(http.StatusFound, "/staff")
}

func HandleStaffEdit(c *gin.Context) {
	dm := getDB(c)
	actor := c.MustGet("user").(*User)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleNotFound(c)
		return
	}

	username := c.PostForm("username")
	role := c.PostForm("role")

	target, err := dm.GetUserByID(id)
	if err == sql.ErrNoRows {
		HandleNotFound(c)
		return
	}
	if err != nil {
		log.Printf("HandleStaffEdit: GetUserByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	if target.Role == "patron" {
		HandleNotFound(c)
		return
	}

	if role != "admin" && role != "staff" {
		setFlash(c, flashKindError, "invalid_role")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	if err := ValidateUsername(username); err != nil {
		setFlash(c, flashKindError, "invalid_username")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	if target.ID == actor.ID && target.Role == "admin" && role != "admin" {
		setFlash(c, flashKindError, "cannot_demote_self")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	if target.Role == "admin" && role != "admin" {
		adminCount, err := dm.CountAdmins()
		if err != nil {
			log.Printf("HandleStaffEdit: CountAdmins: %v", err)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		if adminCount == 1 {
			setFlash(c, flashKindError, "cannot_demote_last_admin")
			c.Redirect(http.StatusFound, "/staff")
			return
		}
	}

	if username != target.Username {
		_, err := dm.GetUserByUsername(username)
		if err == nil {
			setFlash(c, flashKindError, "duplicate_username")
			c.Redirect(http.StatusFound, "/staff")
			return
		}
		if err != sql.ErrNoRows {
			log.Printf("HandleStaffEdit: GetUserByUsername: %v", err)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
	}

	if err := dm.UpdateStaffUser(id, username, role); err != nil {
		log.Printf("HandleStaffEdit: UpdateStaffUser: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	setFlash(c, flashKindSuccess, "staff_updated")
	c.Redirect(http.StatusFound, "/staff")
}

func HandleStaffDelete(c *gin.Context) {
	dm := getDB(c)
	actor := c.MustGet("user").(*User)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleNotFound(c)
		return
	}

	target, err := dm.GetUserByID(id)
	if err == sql.ErrNoRows {
		HandleNotFound(c)
		return
	}
	if err != nil {
		log.Printf("HandleStaffDelete: GetUserByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	if target.Role == "patron" {
		HandleNotFound(c)
		return
	}

	if target.ID == actor.ID {
		setFlash(c, flashKindError, "cannot_delete_self")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	if target.Role == "admin" {
		adminCount, err := dm.CountAdmins()
		if err != nil {
			log.Printf("HandleStaffDelete: CountAdmins: %v", err)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		if adminCount == 1 {
			setFlash(c, flashKindError, "cannot_delete_last_admin")
			c.Redirect(http.StatusFound, "/staff")
			return
		}
	}

	if err := dm.DeleteUser(id); err != nil {
		log.Printf("HandleStaffDelete: DeleteUser: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	setFlash(c, flashKindSuccess, "staff_deleted")
	c.Redirect(http.StatusFound, "/staff")
}

func HandleStaffResetPassword(c *gin.Context) {
	dm := getDB(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleNotFound(c)
		return
	}

	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password_confirm")

	target, err := dm.GetUserByID(id)
	if err == sql.ErrNoRows {
		HandleNotFound(c)
		return
	}
	if err != nil {
		log.Printf("HandleStaffResetPassword: GetUserByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	if target.Role == "patron" {
		HandleNotFound(c)
		return
	}

	if password != passwordConfirm {
		setFlash(c, flashKindError, "password_mismatch")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	if err := ValidatePassword(password); err != nil {
		setFlash(c, flashKindError, "weak_password")
		c.Redirect(http.StatusFound, "/staff")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("HandleStaffResetPassword: bcrypt: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	if err := dm.UpdateUserPassword(id, string(hash)); err != nil {
		log.Printf("HandleStaffResetPassword: UpdateUserPassword: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	setFlash(c, flashKindSuccess, "password_reset")
	c.Redirect(http.StatusFound, "/staff")
}
