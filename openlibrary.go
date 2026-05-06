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

type OpenLibraryBook struct {
	Title       string   `json:"title,omitempty"`
	Authors     []string `json:"authors,omitempty"`
	PublishYear int      `json:"publish_year,omitempty"`
	Publisher   string   `json:"publisher,omitempty"`
	CoverURL    string   `json:"cover_url,omitempty"`
}

type olResponse map[string]olBook

type olBook struct {
	Title       string        `json:"title"`
	Authors     []olAuthor    `json:"authors"`
	Publishers  []olPublisher `json:"publishers"`
	PublishDate string        `json:"publish_date"`
	Cover       olCover       `json:"cover"`
}

type olAuthor struct {
	Name string `json:"name"`
}

type olPublisher struct {
	Name string `json:"name"`
}

type olCover struct {
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
}

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
	}
	for _, a := range b.Authors {
		if a.Name != "" {
			out.Authors = append(out.Authors, a.Name)
		}
	}
	if len(b.Publishers) > 0 {
		out.Publisher = b.Publishers[0].Name
	}
	switch {
	case b.Cover.Large != "":
		out.CoverURL = b.Cover.Large
	case b.Cover.Medium != "":
		out.CoverURL = b.Cover.Medium
	case b.Cover.Small != "":
		out.CoverURL = b.Cover.Small
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
	q.Set("jscmd", "data")
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

	book, ok := payload[bibkey]
	if !ok {
		return nil, ErrOpenLibraryNotFound
	}

	return normalizeOpenLibraryBook(book), nil
}
