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

func TestNormalizeOLAuthorString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Austen, Jane, 1775-1817.", "Jane Austen"},
		{"Herbert, Frank", "Frank Herbert"},
		{"García Márquez, Gabriel, 1927-2014.", "Gabriel García Márquez"},
		{"Tolkien, J. R. R.", "J. R. R. Tolkien"},
		{"Anonymous", "Anonymous"},
		{"Doe, 1900-1980.", "Doe"}, // no first name; only dates after the comma
		{"", ""},
		{"  Austen, Jane, 1775-1817.  ", "Jane Austen"}, // leading/trailing whitespace
		{"Lastname,", "Lastname"},                       // trailing comma, nothing after
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := normalizeOLAuthorString(tc.in); got != tc.want {
				t.Errorf("normalizeOLAuthorString(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestNormalizeOpenLibraryBook_FallsBackToSingularAuthor pins the bug
// fix for OL records that carry authors only under the singular
// `author` field in "Last, First, dates." form. Real-world example:
// the Penguin Classics edition of Pride and Prejudice (ISBN
// 9780141439518) returns "author": ["Austen, Jane, 1775-1817."] with
// no `authors` array, so the previous code dropped the author entirely.
func TestNormalizeOpenLibraryBook_FallsBackToSingularAuthor(t *testing.T) {
	got := normalizeOpenLibraryBook(olBook{
		Title:  "Pride and Prejudice",
		Author: []string{"Austen, Jane, 1775-1817."},
	})
	if len(got.Authors) != 1 || got.Authors[0] != "Jane Austen" {
		t.Errorf("Authors = %v, want [Jane Austen]", got.Authors)
	}
}

// TestNormalizeOpenLibraryBook_StructuredAuthorsWinOverSingular pins
// the precedence rule: when a record carries BOTH `authors` (structured)
// and `author` (string), use the structured form. The structured form
// is the canonical OL shape; the singular field exists mostly on older
// catalog-card-style records and is only used as a fallback.
func TestNormalizeOpenLibraryBook_StructuredAuthorsWinOverSingular(t *testing.T) {
	got := normalizeOpenLibraryBook(olBook{
		Title:   "Both Forms",
		Authors: []olAuthor{{Name: "Canonical Name"}},
		Author:  []string{"Catalog, Form, 1900-2000."},
	})
	if len(got.Authors) != 1 || got.Authors[0] != "Canonical Name" {
		t.Errorf("Authors = %v, want [Canonical Name]", got.Authors)
	}
}

// TestOLDescriptionUnmarshal pins the two real-world JSON shapes
// OL uses for the description field: bare string (older records) and
// typed-text object {type, value} (newer records). Anything else
// (null, number, malformed) must yield an empty string, not an error,
// so a single stray record can't block the whole lookup.
// TestFetchOpenLibraryBook_FallsBackToWorkDescription pins the
// edition->work fallback. When the edition record has no description
// but carries a works ref, FetchOpenLibraryBook must follow the work
// key and use its description. Real-world example: the Dell ed. of
// "The Rule of Four" has no description on its edition record but
// the work record carries the back-cover synopsis.
func TestFetchOpenLibraryBook_FallsBackToWorkDescription(t *testing.T) {
	startFakeOLRouter(t,
		// edition (jscmd=details): no description, but a works ref
		`{
			"ISBN:9780440241355": {
				"details": {
					"title": "The rule of four",
					"authors": [{"name": "Ian Caldwell"}],
					"works": [{"key": "/works/OL8990536W"}]
				}
			}
		}`,
		"", // jscmd=data not configured (won't fire -- authors present)
		map[string]string{
			"/works/OL8990536W": `{
				"description": {"type": "/type/text", "value": "Princeton. Good Friday, 1999. On the eve of graduation..."}
			}`,
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchOpenLibraryBook(ctx, "9780440241355")
	if err != nil {
		t.Fatalf("FetchOpenLibraryBook: %v", err)
	}
	if !strings.Contains(got.Description, "Princeton. Good Friday") {
		t.Errorf("Description = %q, want work-record blurb", got.Description)
	}
}

// TestFetchOpenLibraryBook_WorkFallbackSilentOn404 pins the
// non-fatal-failure contract: when the work record 404s, the edition
// metadata still flows back to the caller with an empty description.
// The handler must not 500 just because the work fetch failed.
func TestFetchOpenLibraryBook_WorkFallbackSilentOn404(t *testing.T) {
	startFakeOLRouter(t,
		`{
			"ISBN:9780440241355": {
				"details": {
					"title": "Sparse Edition",
					"authors": [{"name": "Some Author"}],
					"works": [{"key": "/works/OLDoesNotExistW"}]
				}
			}
		}`,
		"",
		map[string]string{}, // no work bodies -> 404
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchOpenLibraryBook(ctx, "9780440241355")
	if err != nil {
		t.Fatalf("FetchOpenLibraryBook should be non-fatal on work 404; got err=%v", err)
	}
	if got.Title != "Sparse Edition" {
		t.Errorf("Title = %q, want edition metadata preserved", got.Title)
	}
	if got.Description != "" {
		t.Errorf("Description = %q, want empty on work miss", got.Description)
	}
}

// TestFetchOpenLibraryBook_FallsBackToDataAuthors pins the
// edition->jscmd=data author fallback. When the edition record has
// neither structured `authors` nor singular `author`, FetchOpenLibraryBook
// calls jscmd=data, which OL resolves from work-record refs into
// {name, url} objects. Real-world example: the Dell ed. of "The Rule
// of Four" has no author info on the edition; jscmd=data returns
// Ian Caldwell, Dustin Thomason, and Eiko Kakinuma by name.
func TestFetchOpenLibraryBook_FallsBackToDataAuthors(t *testing.T) {
	startFakeOLRouter(t,
		// edition: no authors at all
		`{
			"ISBN:9780440241355": {
				"details": {
					"title": "The rule of four"
				}
			}
		}`,
		// jscmd=data: resolved author names
		`{
			"ISBN:9780440241355": {
				"title": "The rule of four",
				"authors": [
					{"name": "Ian Caldwell"},
					{"name": "Dustin Thomason"},
					{"name": "Eiko Kakinuma"}
				]
			}
		}`,
		map[string]string{},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchOpenLibraryBook(ctx, "9780440241355")
	if err != nil {
		t.Fatalf("FetchOpenLibraryBook: %v", err)
	}
	want := []string{"Ian Caldwell", "Dustin Thomason", "Eiko Kakinuma"}
	if !reflect.DeepEqual(got.Authors, want) {
		t.Errorf("Authors = %v, want %v", got.Authors, want)
	}
}

// TestFetchOpenLibraryBook_DataAuthorsFallbackSilentOn404 pins the
// non-fatal-failure contract for the data-authors fallback. When the
// data call 404s, edition metadata still flows back with empty Authors.
func TestFetchOpenLibraryBook_DataAuthorsFallbackSilentOn404(t *testing.T) {
	startFakeOLRouter(t,
		`{
			"ISBN:9780440241355": {
				"details": {
					"title": "Sparse Edition"
				}
			}
		}`,
		"", // 404 on data
		map[string]string{},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchOpenLibraryBook(ctx, "9780440241355")
	if err != nil {
		t.Fatalf("FetchOpenLibraryBook should be non-fatal on data 404; got err=%v", err)
	}
	if got.Title != "Sparse Edition" {
		t.Errorf("Title = %q, want edition metadata preserved", got.Title)
	}
	if len(got.Authors) != 0 {
		t.Errorf("Authors = %v, want empty on data miss", got.Authors)
	}
}

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
// status code and body, and a t.Cleanup hook to restore openLibraryBaseURL
// and openLibraryHost. Both URLs are swapped to the same fake so a
// work-fallback fetch lands on the same server (where it will fail to
// decode as a work record and be silently swallowed -- safe for tests
// that don't exercise the fallback path).
func fakeOLServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prevBase, prevHost := openLibraryBaseURL, openLibraryHost
	openLibraryBaseURL = srv.URL
	openLibraryHost = srv.URL
	t.Cleanup(func() { openLibraryBaseURL = prevBase; openLibraryHost = prevHost })
	return srv
}

// startFakeOLRouter spins up a single httptest.Server that routes
// requests by path + jscmd query param so tests can exercise the
// multi-call OL flow (jscmd=details, jscmd=data, and /works/<id>.json).
//   - detailsBody: JSON returned for /api/books?jscmd=details
//   - dataBody:    JSON returned for /api/books?jscmd=data (empty
//                  string means the route is unconfigured -- the
//                  handler returns 404).
//   - workBodies:  map of work key ("/works/OL...W") to JSON body.
//                  An empty map means all work requests 404.
func startFakeOLRouter(t *testing.T, detailsBody, dataBody string, workBodies map[string]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/books"):
			body := detailsBody
			if r.URL.Query().Get("jscmd") == "data" {
				body = dataBody
			}
			if body == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		case strings.HasPrefix(r.URL.Path, "/works/"):
			// Path is "/works/OL...W.json"; trim the ".json" suffix to
			// recover the key used by the map.
			key := strings.TrimSuffix(r.URL.Path, ".json")
			body, ok := workBodies[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	prevBase, prevHost := openLibraryBaseURL, openLibraryHost
	openLibraryBaseURL = srv.URL + "/api/books"
	openLibraryHost = srv.URL
	t.Cleanup(func() { openLibraryBaseURL = prevBase; openLibraryHost = prevHost })
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
