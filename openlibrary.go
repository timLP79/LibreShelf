// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// openLibraryBaseURL is a var (not const) so tests can swap it at the
// httptest.Server URL via t.Cleanup(...) without spinning up a real
// network proxy. Production callers never mutate it.
var openLibraryBaseURL = "https://openlibrary.org/api/books"

const (
	openLibraryUserAgent = "LibreShelf/0.1 (+https://github.com/timLP79/LibreShelf)"
	openLibraryTimeout   = 10 * time.Second
)

var ErrOpenLibraryNotFound = errors.New("open library: isbn not found")

// OpenLibraryBook is the prefill payload returned by HandleOpenLibraryLookup.
// Most fields come from Open Library; Description may come from
// Open Library OR from Wikipedia when OL's description is empty/thin.
// DescriptionSource ("openlibrary" or "wikipedia") tells the JS-side
// prefill code which source-label to show in the status banner.
type OpenLibraryBook struct {
	Title             string   `json:"title,omitempty"`
	Authors           []string `json:"authors,omitempty"`
	PublishYear       int      `json:"publish_year,omitempty"`
	Publisher         string   `json:"publisher,omitempty"`
	CoverURL          string   `json:"cover_url,omitempty"`
	Description       string   `json:"description,omitempty"`
	DescriptionSource string   `json:"description_source,omitempty"`
}

// olResponse mirrors the jscmd=details envelope returned by
// https://openlibrary.org/api/books?jscmd=details. Switched from
// jscmd=data because data does not include descriptions; details does
// (under .details.description) and additionally returns cover IDs we
// can format into our preferred -L.jpg URL ourselves.
type olResponse map[string]olEntry

type olEntry struct {
	Details olBook `json:"details"`
}

// olBook is the shape of the `.details` sub-object on the jscmd=details
// response. Several fields differ from the jscmd=data shape this file
// used previously: publishers is []string not []{name string}, the
// covers field is []int (cover IDs we format into URLs ourselves), and
// description appears here at all.
type olBook struct {
	Title       string        `json:"title"`
	Authors     []olAuthor    `json:"authors"`
	Publishers  []string      `json:"publishers"`
	PublishDate string        `json:"publish_date"`
	Covers      []int         `json:"covers"`
	Description olDescription `json:"description"`
}

type olAuthor struct {
	Name string `json:"name"`
}

// olDescription unmarshals OL's description field, which is either a
// bare string (older records) or a typed-text object of the form
// {"type": "/type/text", "value": "..."} (newer records). Both yield
// a plain string for the caller.
type olDescription struct {
	Value string
}

func (d *olDescription) UnmarshalJSON(data []byte) error {
	// Try the typed-text object shape first.
	var obj struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(data, &obj); err == nil && obj.Value != "" {
		d.Value = obj.Value
		return nil
	}
	// Fall through to the bare-string shape.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		d.Value = s
		return nil
	}
	// Anything else (null, number, etc.) -> leave empty, do not error.
	// OL records are heterogeneous; refusing to parse one stray record
	// would block the whole lookup.
	d.Value = ""
	return nil
}

// olCoverURLTemplate builds the large cover URL for a given OL cover
// ID. The covers endpoint takes the raw ID and a size suffix
// (S/M/L). We always request L so the cover-storage pipeline gets
// the highest available resolution.
const olCoverURLTemplate = "https://covers.openlibrary.org/b/id/%d-L.jpg"

var yearRegex = regexp.MustCompile(`\b(1[5-9]\d{2}|20\d{2}|2100)\b`)

func parsePublishYear(s string) int {
	match := yearRegex.FindString(s)
	if match == "" {
		return 0
	}
	n, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return n
}

func stripISBNFormatting(s string) string {
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func normalizeOpenLibraryBook(b olBook) *OpenLibraryBook {
	out := &OpenLibraryBook{
		Title:       b.Title,
		PublishYear: parsePublishYear(b.PublishDate),
		Description: strings.TrimSpace(b.Description.Value),
	}
	for _, a := range b.Authors {
		if a.Name != "" {
			out.Authors = append(out.Authors, a.Name)
		}
	}
	if len(b.Publishers) > 0 {
		out.Publisher = b.Publishers[0]
	}
	if len(b.Covers) > 0 && b.Covers[0] > 0 {
		out.CoverURL = fmt.Sprintf(olCoverURLTemplate, b.Covers[0])
	}
	return out
}

func FetchOpenLibraryBook(ctx context.Context, isbn string) (*OpenLibraryBook, error) {
	bibkey := "ISBN:" + stripISBNFormatting(isbn)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openLibraryBaseURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("bibkeys", bibkey)
	q.Set("format", "json")
	q.Set("jscmd", "details")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("User-Agent", openLibraryUserAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: openLibraryTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open library request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library returned status %d", resp.StatusCode)
	}

	var payload olResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("open library decode: %w", err)
	}

	entry, ok := payload[bibkey]
	if !ok {
		return nil, ErrOpenLibraryNotFound
	}

	return normalizeOpenLibraryBook(entry.Details), nil
}
