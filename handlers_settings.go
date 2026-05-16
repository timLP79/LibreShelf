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
			// Default false, not offlineEnvDefault: the locked state is
			// surfaced via OfflineModeLocked; OfflineMode only drives
			// the toggle when unlocked.
			"OfflineMode":       dm.GetSettingBool("offline_mode", false),
			"OfflineModeLocked": IsOfflineEnvLocked(),
		},
	})
}

func HandleSettingsPost(c *gin.Context) {
	user := c.MustGet("user").(*User)
	dm := getDB(c)

	type setting struct {
		key       string
		field     string
		skipWrite bool
	}
	for _, s := range []setting{
		{"staff_can_import_patrons", "staff_can_import_patrons", false},
		{"offline_mode", "offline_mode", IsOfflineEnvLocked()},
	} {
		if s.skipWrite {
			continue
		}
		value := "false"
		if c.PostForm(s.field) == "on" {
			value = "true"
		}
		if err := dm.SetSetting(s.key, value, user.ID); err != nil {
			log.Printf("settings save failed for %s: %v", s.key, err)
			setFlash(c, flashKindError, "settings_save_failed")
			c.Redirect(http.StatusSeeOther, "/admin/settings")
			return
		}
	}

	setFlash(c, flashKindSuccess, "settings_saved")
	c.Redirect(http.StatusSeeOther, "/admin/settings")
}
