// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParsePublishYear(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"plain year", "1813", 1813},
		{"with month", "January 1925", 1925},
		{"day-month-year", "10 March 1949", 1949},
		{"too old to match (regex floor 1500)", "1499", 0},
		{"too far future (regex ceiling 2100)", "2200", 0},
		{"just-in-bound 1500", "1500", 1500},
		{"bound 2100", "2100", 2100},
		{"no year, just words", "Spring edition", 0},
		{"first match wins", "Reprinted 2010 from 1900 original", 2010},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePublishYear(tc.in); got != tc.want {
				t.Errorf("parsePublishYear(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestStripISBNFormatting(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"9780141439518", "9780141439518"},
		{"978-0-14-143951-8", "9780141439518"},
		{"978 0 14 143951 8", "9780141439518"},
		{"  9780141439518  ", "9780141439518"}, // all spaces stripped, including leading/trailing
		{"", ""},
	}
	for _, tc := range cases {
		if got := stripISBNFormatting(tc.in); got != tc.want {
			t.Errorf("stripISBNFormatting(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeOpenLibraryBook(t *testing.T) {
	t.Run("populated", func(t *testing.T) {
		in := olBook{
			Title:       "Pride and Prejudice",
			PublishDate: "January 1813",
			Authors:     []olAuthor{{Name: "Jane Austen"}, {Name: "Editor"}},
			Publishers:  []string{"Penguin", "Other"},
			Covers:      []int{1234567, 999},
			Description: olDescription{Value: "A romance."},
		}
		want := &OpenLibraryBook{
			Title:       "Pride and Prejudice",
			Authors:     []string{"Jane Austen", "Editor"},
			PublishYear: 1813,
			Publisher:   "Penguin", // first publisher wins
			CoverURL:    "https://covers.openlibrary.org/b/id/1234567-L.jpg",
			Description: "A romance.",
		}
		got := normalizeOpenLibraryBook(in)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("first cover id wins", func(t *testing.T) {
		got := normalizeOpenLibraryBook(olBook{Covers: []int{42, 99}})
		want := "https://covers.openlibrary.org/b/id/42-L.jpg"
		if got.CoverURL != want {
			t.Errorf("CoverURL = %q, want %q", got.CoverURL, want)
		}
	})

	t.Run("zero or empty covers produce no URL", func(t *testing.T) {
		got := normalizeOpenLibraryBook(olBook{Covers: []int{0}})
		if got.CoverURL != "" {
			t.Errorf("CoverURL = %q for zero cover ID, want empty", got.CoverURL)
		}
		got = normalizeOpenLibraryBook(olBook{Covers: nil})
		if got.CoverURL != "" {
			t.Errorf("CoverURL = %q for nil covers, want empty", got.CoverURL)
		}
	})

	t.Run("description whitespace trimmed", func(t *testing.T) {
		got := normalizeOpenLibraryBook(olBook{Description: olDescription{Value: "  \n  A blurb.\n  "}})
		if got.Description != "A blurb." {
			t.Errorf("Description = %q, want %q", got.Description, "A blurb.")
		}
	})

	t.Run("empty author names skipped", func(t *testing.T) {
		got := normalizeOpenLibraryBook(olBook{Authors: []olAuthor{{Name: "A"}, {Name: ""}, {Name: "B"}}})
		want := []string{"A", "B"}
		if !reflect.DeepEqual(got.Authors, want) {
			t.Errorf("Authors = %v, want %v", got.Authors, want)
		}
	})

	t.Run("missing fields default cleanly", func(t *testing.T) {
		got := normalizeOpenLibraryBook(olBook{})
		if got.Title != "" || got.PublishYear != 0 || got.Publisher != "" ||
			got.CoverURL != "" || got.Description != "" {
			t.Errorf("zero-value olBook should normalize to zero-value, got %+v", got)
		}
	})
}

// TestOLDescriptionUnmarshal pins the two real-world JSON shapes
// OL uses for the description field: bare string (older records) and
// typed-text object {type, value} (newer records). Anything else
// (null, number, malformed) must yield an empty string, not an error,
// so a single stray record can't block the whole lookup.
func TestOLDescriptionUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{"bare string", `"A blurb."`, "A blurb."},
		{"typed-text object", `{"type":"/type/text","value":"A typed blurb."}`, "A typed blurb."},
		{"empty string", `""`, ""},
		{"object with empty value falls through to string parse", `{"type":"/type/text","value":""}`, ""},
		{"null is empty, not an error", `null`, ""},
		{"number is empty, not an error", `42`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d olDescription
			if err := d.UnmarshalJSON([]byte(tc.json)); err != nil {
				t.Fatalf("UnmarshalJSON(%q): unexpected error %v", tc.json, err)
			}
			if d.Value != tc.want {
				t.Errorf("got %q, want %q", d.Value, tc.want)
			}
		})
	}
}

// fakeOLServer returns an httptest.Server that responds with the given
// status code and body, and a t.Cleanup hook to restore openLibraryBaseURL.
func fakeOLServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prev := openLibraryBaseURL
	openLibraryBaseURL = srv.URL
	t.Cleanup(func() { openLibraryBaseURL = prev })
	return srv
}

func TestFetchOpenLibraryBook_Success(t *testing.T) {
	body := `{
		"ISBN:9780141439518": {
			"bib_key": "ISBN:9780141439518",
			"details": {
				"title": "Pride and Prejudice",
				"authors": [{"name": "Jane Austen"}],
				"publishers": ["Penguin Classics"],
				"publish_date": "1813",
				"covers": [1234567],
				"description": {"type": "/type/text", "value": "A romance about Elizabeth Bennet."}
			}
		}
	}`
	fakeOLServer(t, http.StatusOK, body)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchOpenLibraryBook(ctx, "978-0-14-143951-8")
	if err != nil {
		t.Fatalf("FetchOpenLibraryBook: %v", err)
	}
	if got.Title != "Pride and Prejudice" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.PublishYear != 1813 {
		t.Errorf("PublishYear = %d", got.PublishYear)
	}
	if got.CoverURL != "https://covers.openlibrary.org/b/id/1234567-L.jpg" {
		t.Errorf("CoverURL = %q", got.CoverURL)
	}
	if got.Description != "A romance about Elizabeth Bennet." {
		t.Errorf("Description = %q", got.Description)
	}
}

// TestFetchOpenLibraryBook_DescriptionAsBareString pins that the older
// OL record shape (description as a plain string) still parses.
func TestFetchOpenLibraryBook_DescriptionAsBareString(t *testing.T) {
	body := `{
		"ISBN:9780141439518": {
			"details": {
				"title": "Old Record",
				"description": "Plain string description."
			}
		}
	}`
	fakeOLServer(t, http.StatusOK, body)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchOpenLibraryBook(ctx, "9780141439518")
	if err != nil {
		t.Fatalf("FetchOpenLibraryBook: %v", err)
	}
	if got.Description != "Plain string description." {
		t.Errorf("Description = %q", got.Description)
	}
}

func TestFetchOpenLibraryBook_NotFound(t *testing.T) {
	// OL returns 200 with an empty object when the ISBN is unknown.
	fakeOLServer(t, http.StatusOK, `{}`)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := FetchOpenLibraryBook(ctx, "0000000000")
	if !errors.Is(err, ErrOpenLibraryNotFound) {
		t.Errorf("err = %v, want ErrOpenLibraryNotFound", err)
	}
}

func TestFetchOpenLibraryBook_HTTPStatus(t *testing.T) {
	fakeOLServer(t, http.StatusInternalServerError, "boom")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := FetchOpenLibraryBook(ctx, "9780141439518")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v, want non-nil mentioning 500", err)
	}
}

func TestFetchOpenLibraryBook_BadJSON(t *testing.T) {
	fakeOLServer(t, http.StatusOK, "not json {{{")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := FetchOpenLibraryBook(ctx, "9780141439518")
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("err = %v, want decode error", err)
	}
}
