// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleSettings(c *gin.Context) {
	dm := getDB(c)
	renderTemplate(c, "admin_settings", gin.H{
		"Title":   "Settings",
		"Error":   readAndClearFlash(c, flashKindError),
		"Success": readAndClearFlash(c, flashKindSuccess),
		"Settings": gin.H{
			"StaffCanImportPatrons": dm.GetSettingBool("staff_can_import_patrons", false),
		},
	})
}

func HandleSettingsPost(c *gin.Context) {
	user := c.MustGet("user").(*User)
	dm := getDB(c)

	enabled := c.PostForm("staff_can_import_patrons") == "on"
	value := "false"
	if enabled {
		value = "true"
	}
	if err := dm.SetSetting("staff_can_import_patrons", value, user.ID); err != nil {
		log.Printf("settings save failed: %v", err)
		setFlash(c, flashKindError, "settings_save_failed")
		c.Redirect(http.StatusSeeOther, "/admin/settings")
		return
	}

	setFlash(c, flashKindSuccess, "settings_saved")
	c.Redirect(http.StatusSeeOther, "/admin/settings")
}
