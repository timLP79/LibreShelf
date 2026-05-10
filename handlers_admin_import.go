// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	importMaxUploadBytes = 10 * 1024 * 1024
	importStashTTL       = 30 * time.Minute
	importDownloadTTL    = 60 * time.Minute
)

type importStashEntry struct {
	Parsed      *ParsedPatronCSV
	AccountMode string
	CreatedAt   time.Time
}

type importDownloadEntry struct {
	Filename  string
	Body      []byte
	CreatedAt time.Time
}

var (
	importStashMu     sync.Mutex
	importStash       = make(map[string]importStashEntry)
	importDownloadsMu sync.Mutex
	importDownloads   = make(map[string]importDownloadEntry)
)

func generateImportToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func sweepImportStashes() {
	cutoff := time.Now().Add(-importStashTTL)
	importStashMu.Lock()
	defer importStashMu.Unlock()
	for k, v := range importStash {
		if v.CreatedAt.Before(cutoff) {
			delete(importStash, k)
		}
	}
}

func sweepImportDownloads() {
	cutoff := time.Now().Add(-importDownloadTTL)
	importDownloadsMu.Lock()
	defer importDownloadsMu.Unlock()
	for k, v := range importDownloads {
		if v.CreatedAt.Before(cutoff) {
			delete(importDownloads, k)
		}
	}
}

func HandlePatronImportForm(c *gin.Context) {
	renderTemplate(c, "admin_patrons_import", gin.H{
		"Title": "Patron Import",
	})
}

func HandlePatronImportPreview(c *gin.Context) {
	renderForm := func(errorMsg string) {
		renderTemplate(c, "admin_patrons_import", gin.H{
			"Title": "Patron Import",
			"Error": errorMsg,
		})
	}

	accountMode := c.PostForm("account_mode")
	if accountMode != "records_only" && accountMode != "with_logins" {
		renderForm("Choose an account mode.")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, importMaxUploadBytes)

	file, _, err := c.Request.FormFile("csv_file")
	if err != nil {
		renderForm("Choose a CSV file to upload (max 10 MB).")
		return
	}
	defer file.Close()

	parsed, err := parsePatronCSV(file)
	if err != nil {
		renderForm("Could not parse the CSV: " + err.Error())
		return
	}

	dm := getDB(c)
	if err := dedupePatronCSV(parsed, dm); err != nil {
		log.Printf("dedupe failed: %v", err)
		renderForm("Internal error during duplicate detection.")
		return
	}

	sweepImportStashes()
	token, err := generateImportToken()
	if err != nil {
		log.Printf("generate import token: %v", err)
		renderForm("Internal error.")
		return
	}

	importStashMu.Lock()
	importStash[token] = importStashEntry{
		Parsed:      parsed,
		AccountMode: accountMode,
		CreatedAt:   time.Now(),
	}
	importStashMu.Unlock()

	var insertable, dupExternal, dupEmail, emptyName, malformed int
	for _, r := range parsed.Rows {
		switch {
		case r.Insert:
			insertable++
		case r.SkipReason == "Name is required.":
			emptyName++
		case strings.HasPrefix(r.SkipReason, "External ID"):
			dupExternal++
		case strings.HasPrefix(r.SkipReason, "Email"):
			dupEmail++
		default:
			malformed++
		}
	}

	existingCount, err := dm.CountPatrons()
	if err != nil {
		log.Printf("count patrons: %v", err)
	}

	var sample []ParsedPatronRow
	for _, r := range parsed.Rows {
		if r.Insert && len(sample) < 10 {
			sample = append(sample, r)
		}
	}

	csrfToken, _ := c.Get("csrfToken")
	renderTemplate(c, "admin_patrons_import_preview", gin.H{
		"Title":           "Patron Import: Preview",
		"CSRFToken":       csrfToken,
		"Token":           token,
		"AccountMode":     accountMode,
		"Insertable":      insertable,
		"DupExternal":     dupExternal,
		"DupEmail":        dupEmail,
		"EmptyName":       emptyName,
		"Malformed":       malformed,
		"Total":           len(parsed.Rows),
		"ExistingCount":   existingCount,
		"Sample":          sample,
		"AllRows":         parsed.Rows,
		"StandardColumns": parsed.StandardColumns,
		"MetadataColumns": parsed.MetadataColumns,
	})
}

func HandlePatronImportCommit(c *gin.Context) {
	token := c.PostForm("token")
	if token == "" {
		c.String(http.StatusBadRequest, "Missing token")
		return
	}

	importStashMu.Lock()
	entry, ok := importStash[token]
	if ok {
		delete(importStash, token)
	}
	importStashMu.Unlock()

	if !ok {
		c.String(http.StatusBadRequest, "Import session expired or already used. Please re-upload.")
		return
	}

	dm := getDB(c)

	type credentialRow struct {
		Name         string
		Username     string
		TempPassword string
	}
	type errorRow struct {
		LineNumber int
		Reason     string
	}

	var credentials []credentialRow
	var errors []errorRow
	var inserted int

	for _, row := range entry.Parsed.Rows {
		if !row.Insert {
			errors = append(errors, errorRow{LineNumber: row.LineNumber, Reason: row.SkipReason})
			continue
		}

		metadataJSON, err := buildMetadataJSON(row.ExternalID, row.Metadata)
		if err != nil {
			errors = append(errors, errorRow{LineNumber: row.LineNumber, Reason: "encode metadata: " + err.Error()})
			continue
		}

		if entry.AccountMode == "with_logins" {
			_, _, username, tempPassword, err := dm.CreatePatronWithLogin(row.Name, row.Email, row.Phone, metadataJSON)
			if err != nil {
				errors = append(errors, errorRow{LineNumber: row.LineNumber, Reason: err.Error()})
				continue
			}
			credentials = append(credentials, credentialRow{Name: row.Name, Username: username, TempPassword: tempPassword})
			inserted++
		} else {
			if _, err := dm.CreatePatronNoLogin(row.Name, row.Email, row.Phone, metadataJSON); err != nil {
				errors = append(errors, errorRow{LineNumber: row.LineNumber, Reason: err.Error()})
				continue
			}
			inserted++
		}
	}

	var credentialsToken, errorsToken string
	if len(credentials) > 0 {
		rows := [][]string{{"Name", "Username", "Temporary Password"}}
		for _, cred := range credentials {
			rows = append(rows, []string{cred.Name, cred.Username, cred.TempPassword})
		}
		credentialsToken = stashCSVDownload(rows, "patron_credentials.csv")
	}
	if len(errors) > 0 {
		rows := [][]string{{"Line Number", "Reason"}}
		for _, e := range errors {
			rows = append(rows, []string{strconv.Itoa(e.LineNumber), e.Reason})
		}
		errorsToken = stashCSVDownload(rows, "patron_import_errors.csv")
	}
	sweepImportDownloads()

	renderTemplate(c, "admin_patrons_import_result", gin.H{
		"Title":            "Patron Import: Done",
		"Inserted":         inserted,
		"ErrorCount":       len(errors),
		"CredentialsToken": credentialsToken,
		"ErrorsToken":      errorsToken,
		"AccountMode":      entry.AccountMode,
	})
}

func stashCSVDownload(rows [][]string, filename string) string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	for _, r := range rows {
		defanged := make([]string, len(r))
		for i, cell := range r {
			defanged[i] = defangCSVCell(cell)
		}
		_ = w.Write(defanged)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		log.Printf("write %s: %v", filename, err)
		return ""
	}
	token, err := generateImportToken()
	if err != nil {
		log.Printf("generate download token for %s: %v", filename, err)
		return ""
	}
	importDownloadsMu.Lock()
	importDownloads[token] = importDownloadEntry{
		Filename:  filename,
		Body:      buf.Bytes(),
		CreatedAt: time.Now(),
	}
	importDownloadsMu.Unlock()
	return token
}

func HandleImportDownload(c *gin.Context) {
	token := c.Param("token")
	importDownloadsMu.Lock()
	entry, ok := importDownloads[token]
	if ok {
		delete(importDownloads, token)
	}
	importDownloadsMu.Unlock()
	if !ok {
		c.String(http.StatusNotFound, "Download expired or already used.")
		return
	}
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("Pragma", "no-cache")
	c.Header("Content-Disposition", "attachment; filename="+entry.Filename)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", entry.Body)
}

// defangCSVCell prefixes a single quote on cells that would otherwise be
// interpreted as a formula by Excel / LibreOffice / Sheets when the CSV
// is opened. Defending against CSV / formula injection where attacker-
// controlled patron Name cells could exfiltrate adjacent plaintext temp
// passwords via =HYPERLINK / =IMPORTXML / =WEBSERVICE.
func defangCSVCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}
