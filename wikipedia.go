// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// wikipediaSummaryURL is the REST summary endpoint. Title is appended
// to the URL as a path segment (no trailing slash on the base). Both
// URLs are vars so tests can swap to httptest.Server URLs.
var (
	wikipediaSummaryURL = "https://en.wikipedia.org/api/rest_v1/page/summary/"
	wikipediaSearchURL  = "https://en.wikipedia.org/w/api.php"
)

const (
	// Wikipedia's API policy expects an identifiable User-Agent.
	// See https://en.wikipedia.org/api/rest_v1/ -- anonymous traffic
	// gets aggressively rate-limited.
	wikipediaUserAgent = "LibreShelf/0.1 (+https://github.com/timLP79/LibreShelf)"
	wikipediaTimeout   = 10 * time.Second

	// Max number of opensearch candidates to try before giving up on
	// disambiguation. Five is plenty -- Wikipedia's prefix search ranks
	// the most-linked page first, so an off-by-one mismatch is rare.
	wikipediaMaxCandidates = 5
)

// wikipediaSummary is the JSON shape returned by /api/rest_v1/page/summary/<title>.
// Only the fields we actually use are listed.
type wikipediaSummary struct {
	Type        string `json:"type"`        // "standard", "disambiguation", "no-extract", etc.
	Title       string `json:"title"`
	Description string `json:"description"` // short label, e.g. "1965 novel by Frank Herbert"
	Extract     string `json:"extract"`     // article lead, plain text
}

// FetchWikipediaSummary returns the Wikipedia lead-section extract for
// a book, or an empty string when no acceptable match is found.
//
// Strategy:
//  1. Direct title lookup against /api/rest_v1/page/summary/<title>.
//  2. If the direct hit is a disambiguation, missing, or fails the
//     author-match guard, fall back to opensearch with
//     "<title> <first author>" and walk the top results until one
//     passes the guard.
//
// The author-match guard requires the Wikipedia extract or description
// to contain at least one supplied author name (case-insensitive
// substring). This protects against latching onto an unrelated topic
// with a similar title -- e.g. "Dune" landing on the franchise rather
// than the novel.
//
// Errors are returned for transport-level failures the caller may want
// to log. "No match" is not an error; the caller checks the returned
// string for non-empty.
func FetchWikipediaSummary(ctx context.Context, title string, authors []string) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", nil
	}

	// Attempt 1: direct title lookup.
	sum, err := fetchWikipediaSummaryByTitle(ctx, title)
	if err == nil && sum.Type == "standard" && sum.Extract != "" && extractMatchesAuthor(sum, authors) {
		return strings.TrimSpace(sum.Extract), nil
	}

	// Attempt 2: opensearch disambiguation. We need at least one
	// author to query against and to gate the result -- without one,
	// we cannot tell the right article from a similarly-named topic.
	firstAuthor := firstNonEmpty(authors)
	if firstAuthor == "" {
		return "", nil
	}

	candidates, err := wikipediaOpenSearch(ctx, title+" "+firstAuthor)
	if err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		candSum, err := fetchWikipediaSummaryByTitle(ctx, candidate)
		if err != nil {
			continue
		}
		if candSum.Type != "standard" || candSum.Extract == "" {
			continue
		}
		if !extractMatchesAuthor(candSum, authors) {
			continue
		}
		return strings.TrimSpace(candSum.Extract), nil
	}
	return "", nil
}

// fetchWikipediaSummaryByTitle calls /api/rest_v1/page/summary/<title>.
// A 404 returns a zero-value summary (no error) so the caller can
// proceed to opensearch; other non-200 statuses surface as errors so
// the operator can investigate transient API trouble.
func fetchWikipediaSummaryByTitle(ctx context.Context, title string) (*wikipediaSummary, error) {
	// Wikipedia REST encodes spaces as underscores in the path segment.
	pathTitle := url.PathEscape(strings.ReplaceAll(strings.TrimSpace(title), " ", "_"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wikipediaSummaryURL+pathTitle, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", wikipediaUserAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: wikipediaTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wikipedia summary request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &wikipediaSummary{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikipedia summary returned status %d", resp.StatusCode)
	}

	var sum wikipediaSummary
	if err := json.NewDecoder(resp.Body).Decode(&sum); err != nil {
		return nil, fmt.Errorf("wikipedia summary decode: %w", err)
	}
	return &sum, nil
}

// wikipediaOpenSearch returns up to wikipediaMaxCandidates title
// suggestions for the query. The opensearch endpoint returns a
// four-element JSON array: [query, [titles], [descriptions], [urls]].
// We only need the titles slice.
func wikipediaOpenSearch(ctx context.Context, query string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wikipediaSearchURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("action", "opensearch")
	q.Set("search", query)
	q.Set("limit", fmt.Sprintf("%d", wikipediaMaxCandidates))
	q.Set("namespace", "0") // main article namespace only; skip Talk:, User:, etc.
	q.Set("format", "json")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("User-Agent", wikipediaUserAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: wikipediaTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wikipedia opensearch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikipedia opensearch returned status %d", resp.StatusCode)
	}

	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("wikipedia opensearch decode: %w", err)
	}
	if len(raw) < 2 {
		return nil, nil
	}
	var titles []string
	if err := json.Unmarshal(raw[1], &titles); err != nil {
		return nil, fmt.Errorf("wikipedia opensearch titles: %w", err)
	}
	return titles, nil
}

// extractMatchesAuthor reports whether the Wikipedia summary mentions
// at least one of the supplied authors. Substring match on the
// lowercase extract + description. When the authors slice is empty,
// the guard is skipped (returns true) -- but FetchWikipediaSummary
// refuses to even call opensearch without an author, so the only path
// that reaches "no authors" is the direct title hit, where Wikipedia
// returning a "standard" article with non-empty extract is treated as
// authoritative on its own.
func extractMatchesAuthor(sum *wikipediaSummary, authors []string) bool {
	if len(authors) == 0 {
		return true
	}
	haystack := strings.ToLower(sum.Extract + " " + sum.Description)
	for _, a := range authors {
		a = strings.TrimSpace(strings.ToLower(a))
		if a == "" {
			continue
		}
		if strings.Contains(haystack, a) {
			return true
		}
	}
	return false
}

func firstNonEmpty(ss []string) string {
	for _, s := range ss {
		if t := strings.TrimSpace(s); t != "" {
			return t
		}
	}
	return ""
}
