// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ErrGoogleBooksNotFound is returned when the GB volumes endpoint
// responds with an empty items list. This is a normal outcome for ISBNs
// GB does not catalog, and callers should treat it as a no-op rather
// than an upstream failure.
var ErrGoogleBooksNotFound = errors.New("google books: no results for ISBN")

// ErrGoogleBooksDisabled is returned when GOOGLE_BOOKS_API_KEY is unset.
// Callers should treat this as a non-error short-circuit: the GB chain
// is disabled, OL data is returned alone.
var ErrGoogleBooksDisabled = errors.New("google books: GOOGLE_BOOKS_API_KEY is not set")

// Package-level state. Captured once at startup. Tests override via
// withGoogleBooksBaseURL and withGoogleBooksAPIKey helpers.
var (
	googleBooksBaseURL = "https://www.googleapis.com/books/v1/volumes"
	googleBooksAPIKey  string
	googleBooksTimeout = 5 * time.Second
)

func init() {
	googleBooksAPIKey = os.Getenv("GOOGLE_BOOKS_API_KEY")
}

// gbResponse mirrors the volumes search envelope returned by
// https://www.googleapis.com/books/v1/volumes?q=isbn:<isbn>&key=<key>.
// Only the fields we read are declared; unknown fields are ignored.
type gbResponse struct {
	Items []gbItem `json:"items"`
}

type gbItem struct {
	VolumeInfo gbVolumeInfo `json:"volumeInfo"`
}

type gbVolumeInfo struct {
	Title         string       `json:"title"`
	Authors       []string     `json:"authors"`
	Publisher     string       `json:"publisher"`
	PublishedDate string       `json:"publishedDate"`
	Description   string       `json:"description"`
	ImageLinks    gbImageLinks `json:"imageLinks"`
}

type gbImageLinks struct {
	Thumbnail      string `json:"thumbnail"`
	SmallThumbnail string `json:"smallThumbnail"`
}

// IsGoogleBooksConfigured returns true when GOOGLE_BOOKS_API_KEY was
// set at startup. The merge logic in FetchOpenLibraryBook reads this
// to decide whether to fan out to GB; the attribution footer reads it
// to decide whether to credit Google Books.
func IsGoogleBooksConfigured() bool {
	return googleBooksAPIKey != ""
}

// FetchByISBN queries the Google Books volumes API for the given ISBN.
// Returns ErrGoogleBooksDisabled if GOOGLE_BOOKS_API_KEY is unset.
// Returns ErrGoogleBooksNotFound if GB has no record for the ISBN.
// All other errors (network, decode, non-2xx) are wrapped and returned
// as opaque errors; callers should treat them as "GB unavailable for
// this lookup" and proceed with OL-only data.
func FetchByISBN(ctx context.Context, isbn string) (*BookPrefill, error) {
	if googleBooksAPIKey == "" {
		return nil, ErrGoogleBooksDisabled
	}

	cleaned := stripISBNFormatting(isbn)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleBooksBaseURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("q", "isbn:"+cleaned)
	q.Set("key", googleBooksAPIKey)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", openLibraryUserAgent)

	client := &http.Client{Timeout: googleBooksTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google books request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google books returned status %d", resp.StatusCode)
	}

	var payload gbResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("google books decode: %w", err)
	}

	if len(payload.Items) == 0 {
		return nil, ErrGoogleBooksNotFound
	}

	return normalizeGoogleBook(payload.Items[0].VolumeInfo), nil
}

// normalizeGoogleBook maps a Google Books volumeInfo object to the
// shared BookPrefill shape. Per-field source labels are NOT set here;
// the merge step (mergePrefill) decides which fields' sources to
// stamp based on whether OL also had data.
func normalizeGoogleBook(v gbVolumeInfo) *BookPrefill {
	b := &BookPrefill{
		Title:       strings.TrimSpace(v.Title),
		Authors:     v.Authors,
		Publisher:   strings.TrimSpace(v.Publisher),
		Description: strings.TrimSpace(v.Description),
	}
	if v.PublishedDate != "" {
		// publishedDate may be "2004", "2004-05", or "2004-05-12". Take
		// the first 4 characters and parse as int. Any non-numeric or
		// out-of-range value yields a zero PublishYear, which the merge
		// step treats as "no value."
		if len(v.PublishedDate) >= 4 {
			if y, err := strconv.Atoi(v.PublishedDate[:4]); err == nil && y >= 1500 && y <= 2100 {
				b.PublishYear = y
			}
		}
	}
	if v.ImageLinks.Thumbnail != "" {
		// Google returns http:// for thumbnails. Upgrade to HTTPS so the
		// downstream cover-download path does not get mixed-content
		// warnings or refusals.
		b.CoverURL = strings.Replace(v.ImageLinks.Thumbnail, "http://", "https://", 1)
	}
	return b
}
