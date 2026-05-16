// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// withGoogleBooksBaseURL swaps the package-level base URL for the
// duration of the test and restores it on Cleanup. Tests should never
// hit the real Google Books endpoint.
func withGoogleBooksBaseURL(t *testing.T, url string) {
	t.Helper()
	prior := googleBooksBaseURL
	googleBooksBaseURL = url
	t.Cleanup(func() { googleBooksBaseURL = prior })
}

// withGoogleBooksAPIKey swaps the package-level API key for the
// duration of the test. The key is captured at init() from env; tests
// override it here without touching the env.
func withGoogleBooksAPIKey(t *testing.T, key string) {
	t.Helper()
	prior := googleBooksAPIKey
	googleBooksAPIKey = key
	t.Cleanup(func() { googleBooksAPIKey = prior })
}

func startFakeGBServer(t *testing.T, body string, status int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	withGoogleBooksBaseURL(t, srv.URL)
	withGoogleBooksAPIKey(t, "test-key")
}

const gbFullPayload = `{
  "items": [{
    "volumeInfo": {
      "title": "Dune",
      "authors": ["Frank Herbert"],
      "publisher": "Ace Books",
      "publishedDate": "1965",
      "description": "Set on the desert planet Arrakis, Dune is the story of the boy Paul Atreides.",
      "imageLinks": {
        "thumbnail": "http://books.google.com/books/content?id=B1hSG45JCX4C&printsec=frontcover&img=1&zoom=1"
      }
    }
  }]
}`

const gbCoverOnlyPayload = `{
  "items": [{
    "volumeInfo": {
      "imageLinks": {
        "thumbnail": "http://books.google.com/books/content?id=X&img=1&zoom=1"
      }
    }
  }]
}`

const gbDescOnlyPayload = `{
  "items": [{
    "volumeInfo": {
      "description": "A novel."
    }
  }]
}`

const gbEmptyItemsPayload = `{"totalItems": 0}`

func TestFetchByISBN_HappyFull(t *testing.T) {
	startFakeGBServer(t, gbFullPayload, http.StatusOK)
	got, err := FetchByISBN(context.Background(), "9780441013593")
	if err != nil {
		t.Fatalf("FetchByISBN: %v", err)
	}
	if got.Title != "Dune" {
		t.Errorf("title: want %q, got %q", "Dune", got.Title)
	}
	if len(got.Authors) != 1 || got.Authors[0] != "Frank Herbert" {
		t.Errorf("authors: want [Frank Herbert], got %v", got.Authors)
	}
	if got.Publisher != "Ace Books" {
		t.Errorf("publisher: want %q, got %q", "Ace Books", got.Publisher)
	}
	if got.PublishYear != 1965 {
		t.Errorf("publish_year: want 1965, got %d", got.PublishYear)
	}
	if got.Description == "" {
		t.Errorf("description should be populated")
	}
	if got.CoverURL == "" {
		t.Errorf("cover_url should be populated")
	}
	// Cover URL must be upgraded to HTTPS (Google returns http://).
	if got.CoverURL[:8] != "https://" {
		t.Errorf("cover_url should be HTTPS, got %q", got.CoverURL)
	}
}

func TestFetchByISBN_HappyCoverOnly(t *testing.T) {
	startFakeGBServer(t, gbCoverOnlyPayload, http.StatusOK)
	got, err := FetchByISBN(context.Background(), "9780000000000")
	if err != nil {
		t.Fatalf("FetchByISBN: %v", err)
	}
	if got.CoverURL == "" {
		t.Errorf("cover_url should be populated, got empty")
	}
	if got.Title != "" || got.Description != "" {
		t.Errorf("title and description should be empty, got title=%q desc=%q", got.Title, got.Description)
	}
}

func TestFetchByISBN_HappyDescOnly(t *testing.T) {
	startFakeGBServer(t, gbDescOnlyPayload, http.StatusOK)
	got, err := FetchByISBN(context.Background(), "9780000000000")
	if err != nil {
		t.Fatalf("FetchByISBN: %v", err)
	}
	if got.Description != "A novel." {
		t.Errorf("description: want %q, got %q", "A novel.", got.Description)
	}
	if got.CoverURL != "" {
		t.Errorf("cover_url should be empty, got %q", got.CoverURL)
	}
}

func TestFetchByISBN_NoResults(t *testing.T) {
	startFakeGBServer(t, gbEmptyItemsPayload, http.StatusOK)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if !errors.Is(err, ErrGoogleBooksNotFound) {
		t.Errorf("want ErrGoogleBooksNotFound, got %v", err)
	}
}

func TestFetchByISBN_4xx(t *testing.T) {
	startFakeGBServer(t, `{"error":"bad request"}`, http.StatusBadRequest)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if err == nil {
		t.Errorf("want error on 400, got nil")
	}
	if errors.Is(err, ErrGoogleBooksNotFound) {
		t.Errorf("400 should not map to ErrGoogleBooksNotFound, got %v", err)
	}
}

func TestFetchByISBN_5xx(t *testing.T) {
	startFakeGBServer(t, `{}`, http.StatusInternalServerError)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if err == nil {
		t.Errorf("want error on 500, got nil")
	}
}

func TestFetchByISBN_RateLimit(t *testing.T) {
	startFakeGBServer(t, `{"error":"quota exceeded"}`, http.StatusTooManyRequests)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if err == nil {
		t.Errorf("want error on 429, got nil")
	}
}

func TestFetchByISBN_MalformedJSON(t *testing.T) {
	startFakeGBServer(t, `{not valid json`, http.StatusOK)
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if err == nil {
		t.Errorf("want decode error, got nil")
	}
}

func TestFetchByISBN_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(gbFullPayload))
	}))
	t.Cleanup(srv.Close)
	withGoogleBooksBaseURL(t, srv.URL)
	withGoogleBooksAPIKey(t, "test-key")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := FetchByISBN(ctx, "9780000000000")
	if err == nil {
		t.Errorf("want context timeout error, got nil")
	}
}

func TestFetchByISBN_NoKey_ReturnsSentinel(t *testing.T) {
	withGoogleBooksAPIKey(t, "")
	_, err := FetchByISBN(context.Background(), "9780000000000")
	if !errors.Is(err, ErrGoogleBooksDisabled) {
		t.Errorf("want ErrGoogleBooksDisabled, got %v", err)
	}
}

// Defensive: confirm the test fake honors the published year regardless of
// whether the Google Books field is a bare year or a YYYY-MM-DD string.
func TestFetchByISBN_PublishedDateMonthDay(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"volumeInfo": map[string]any{
				"title":         "X",
				"publishedDate": "2004-05-12",
			}},
		},
	})
	startFakeGBServer(t, string(body), http.StatusOK)
	got, err := FetchByISBN(context.Background(), "9780000000000")
	if err != nil {
		t.Fatalf("FetchByISBN: %v", err)
	}
	if got.PublishYear != 2004 {
		t.Errorf("publish_year: want 2004, got %d", got.PublishYear)
	}
}
