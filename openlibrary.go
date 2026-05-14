// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// openLibraryBaseURL and openLibraryHost are vars (not const) so
// tests can swap them at the httptest.Server URL via t.Cleanup(...)
// without spinning up a real network proxy. Production callers never
// mutate them.
//
// openLibraryBaseURL is the Books API endpoint (/api/books). openLibraryHost
// is the host root, used to construct /works/X.json fetches for the
// work-description fallback.
var (
	openLibraryBaseURL = "https://openlibrary.org/api/books"
	openLibraryHost    = "https://openlibrary.org"
)

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
// response. Several fields differ from the jscmd=data shape: publishers
// is []string not []{name string}, the covers field is []int (cover
// IDs we format into URLs ourselves), and description appears here
// at all (data does not return descriptions).
//
// OL is inconsistent about how it credits authors. Some records have
// `authors`: an array of {key, name} objects in "First Last" form.
// Others have `author`: a flat string array in catalog-card form like
// "Austen, Jane, 1775-1817." Many sparse edition records (e.g. Dell
// paperbacks) have NEITHER -- the author refs live only on the work
// record. We read both edition shapes and fall back to a second
// jscmd=data call when the edition is sparse.
type olBook struct {
	Title       string        `json:"title"`
	Authors     []olAuthor    `json:"authors"`
	Author      []string      `json:"author"`
	Publishers  []string      `json:"publishers"`
	PublishDate string        `json:"publish_date"`
	Covers      []int         `json:"covers"`
	Description olDescription `json:"description"`
	Works       []olWorkRef   `json:"works"`
}

type olAuthor struct {
	Name string `json:"name"`
}

// olWorkRef points at the abstract "work" record for an edition.
// Edition records carry one entry: works: [{"key": "/works/OL...W"}].
// We follow this key when the edition itself is missing a description.
type olWorkRef struct {
	Key string `json:"key"`
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
	// Prefer the structured `authors` array when present. Fall back to
	// `author` (singular) for records that only carry the catalog-card
	// form like "Austen, Jane, 1775-1817." -- those need flipping to
	// "First Last" before they're useful in our form.
	for _, a := range b.Authors {
		if a.Name != "" {
			out.Authors = append(out.Authors, a.Name)
		}
	}
	if len(out.Authors) == 0 {
		for _, raw := range b.Author {
			if name := normalizeOLAuthorString(raw); name != "" {
				out.Authors = append(out.Authors, name)
			}
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

// normalizeOLAuthorString flips OL's catalog-card author form
// ("Austen, Jane, 1775-1817.") into "First Last" and drops the trailing
// date range. Algorithm:
//  1. Strip trailing period.
//  2. Split on the first comma.
//  3. The portion after the first comma may itself contain a date or
//     a role marker, separated by another comma. Take everything up to
//     that next comma as the first-name segment.
//  4. If the first-name segment looks like a date range (begins with a
//     digit), there is no useful first name; return the last-name
//     segment as-is.
//  5. Otherwise return "<first> <last>".
//
// Records with no comma (e.g. "Anonymous") pass through unchanged.
func normalizeOLAuthorString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Drop a trailing period only when the char before it is a digit,
	// i.e. it terminates a date range. A period after an initial
	// (e.g. "Tolkien, J. R. R.") must survive.
	if len(s) >= 2 && s[len(s)-1] == '.' && s[len(s)-2] >= '0' && s[len(s)-2] <= '9' {
		s = strings.TrimSpace(s[:len(s)-1])
	}

	commaIdx := strings.Index(s, ",")
	if commaIdx == -1 {
		return s
	}

	last := strings.TrimSpace(s[:commaIdx])
	rest := strings.TrimSpace(s[commaIdx+1:])
	if rest == "" {
		return last
	}

	// Take the rest up to the next comma so a trailing date or role
	// suffix is dropped.
	if next := strings.Index(rest, ","); next >= 0 {
		rest = strings.TrimSpace(rest[:next])
	}

	if rest == "" {
		return last
	}
	if rest[0] >= '0' && rest[0] <= '9' {
		// The "first name" slot is actually a date range. Author has
		// only a last name in the record.
		return last
	}
	return rest + " " + last
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

	book := normalizeOpenLibraryBook(entry.Details)

	// Fallback 1: when the edition record has no description (common
	// for sparse paperback editions like Dell), follow the work
	// reference and pull from there. The work record often carries
	// the back-cover-style synopsis that staff want to prefill.
	if book.Description == "" && len(entry.Details.Works) > 0 {
		workKey := entry.Details.Works[0].Key
		if desc, err := fetchOLWorkDescription(ctx, workKey); err != nil {
			// Non-fatal: log and continue with empty description.
			// The edition metadata still flows back to the client.
			log.Printf("openlibrary: work description fetch for %s: %v", workKey, err)
		} else if desc != "" {
			book.Description = desc
		}
	}

	// Fallback 2: when the edition record has no authors (neither
	// structured nor catalog-card form), call jscmd=data which OL
	// resolves to the work's author records and returns by name.
	// This recovers authors for cases like the Dell ed. of
	// "The Rule of Four" where details has no author info at all.
	if len(book.Authors) == 0 {
		if names, err := fetchOLDataAuthors(ctx, bibkey); err != nil {
			log.Printf("openlibrary: data-authors fetch for %s: %v", bibkey, err)
		} else {
			book.Authors = names
		}
	}

	return book, nil
}

// fetchOLWorkDescription pulls the description field off an OL work
// record. workKey is the path-style form OL returns from the edition,
// e.g. "/works/OL8990536W"; we fetch "<host>/<workKey>.json".
//
// Returns "" without error if the work has no description, or if the
// fetch fails in a way we want to silently swallow (4xx, 5xx, network
// error). The edition data still gets returned to the client either
// way; this is best-effort enrichment.
func fetchOLWorkDescription(ctx context.Context, workKey string) (string, error) {
	if workKey == "" {
		return "", nil
	}
	endpoint := openLibraryHost + workKey + ".json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", openLibraryUserAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: openLibraryTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("open library work request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("open library work returned status %d", resp.StatusCode)
	}

	// Work records use the same string-or-{type,value} shape for
	// description; reuse our olDescription unmarshaler.
	var work struct {
		Description olDescription `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&work); err != nil {
		return "", fmt.Errorf("open library work decode: %w", err)
	}
	return strings.TrimSpace(work.Description.Value), nil
}

// fetchOLDataAuthors makes a second call to the OL Books API with
// jscmd=data and returns the resolved author names. jscmd=data
// resolves work-record author refs into {name, url} pairs, which is
// exactly what we need when jscmd=details has no authors. Most
// editions only need one call; this fallback fires when details is
// sparse.
func fetchOLDataAuthors(ctx context.Context, bibkey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openLibraryBaseURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("bibkeys", bibkey)
	q.Set("format", "json")
	q.Set("jscmd", "data")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("User-Agent", openLibraryUserAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: openLibraryTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open library data request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library data returned status %d", resp.StatusCode)
	}

	// jscmd=data uses [{"name": "..."}] shape inline.
	var payload map[string]struct {
		Authors []olAuthor `json:"authors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("open library data decode: %w", err)
	}
	entry, ok := payload[bibkey]
	if !ok {
		return nil, nil
	}
	out := make([]string, 0, len(entry.Authors))
	for _, a := range entry.Authors {
		if a.Name != "" {
			out = append(out, a.Name)
		}
	}
	return out, nil
}
