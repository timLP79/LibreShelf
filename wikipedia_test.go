// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeWikipedia spins up a single httptest.Server that handles both
// the REST summary endpoint and the opensearch endpoint, dispatching
// by URL path. summaryByTitle maps the Wikipedia article title (with
// underscores) to the JSON body to return; titles not in the map
// get a 404. openSearchHits is the titles slice returned by the
// opensearch endpoint, regardless of query.
type fakeWiki struct {
	summaryByTitle map[string]string
	openSearchHits []string
}

func startFakeWikipedia(t *testing.T, f fakeWiki) {
	t.Helper()

	prevSummary := wikipediaSummaryURL
	prevSearch := wikipediaSearchURL

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// REST summary requests go through a path prefix; the title is
		// the last segment.
		if strings.HasPrefix(r.URL.Path, "/api/rest_v1/page/summary/") {
			title := strings.TrimPrefix(r.URL.Path, "/api/rest_v1/page/summary/")
			body, ok := f.summaryByTitle[title]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
			return
		}
		// Anything else is opensearch on /w/api.php. Return the
		// configured titles slice in the standard 4-array shape.
		raw, _ := json.Marshal([]any{
			r.URL.Query().Get("search"),
			f.openSearchHits,
			make([]string, len(f.openSearchHits)),
			make([]string, len(f.openSearchHits)),
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(srv.Close)

	wikipediaSummaryURL = srv.URL + "/api/rest_v1/page/summary/"
	wikipediaSearchURL = srv.URL + "/w/api.php"
	t.Cleanup(func() {
		wikipediaSummaryURL = prevSummary
		wikipediaSearchURL = prevSearch
	})
}

func TestFetchWikipediaSummary_DirectTitleHit(t *testing.T) {
	startFakeWikipedia(t, fakeWiki{
		summaryByTitle: map[string]string{
			"Pride_and_Prejudice": `{
				"type": "standard",
				"title": "Pride and Prejudice",
				"description": "1813 novel by Jane Austen",
				"extract": "Pride and Prejudice is an 1813 novel of manners by Jane Austen."
			}`,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchWikipediaSummary(ctx, "Pride and Prejudice", []string{"Jane Austen"})
	if err != nil {
		t.Fatalf("FetchWikipediaSummary: %v", err)
	}
	if !strings.Contains(got, "Jane Austen") {
		t.Errorf("extract should contain author; got %q", got)
	}
}

func TestFetchWikipediaSummary_AuthorMismatchRejected(t *testing.T) {
	// Wikipedia article exists for the title but the extract mentions
	// a different author (or none) -- we must reject rather than
	// import an unrelated topic's summary.
	startFakeWikipedia(t, fakeWiki{
		summaryByTitle: map[string]string{
			"Dune": `{
				"type": "standard",
				"title": "Dune",
				"description": "Sandy terrain landform",
				"extract": "A dune is a landform composed of wind- or water-driven sand."
			}`,
		},
		openSearchHits: []string{}, // no fallback candidates
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchWikipediaSummary(ctx, "Dune", []string{"Frank Herbert"})
	if err != nil {
		t.Fatalf("FetchWikipediaSummary: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty result (author mismatch should reject); got %q", got)
	}
}

func TestFetchWikipediaSummary_DisambiguationFallsThroughToOpenSearch(t *testing.T) {
	// Direct hit on "Dune" is a disambiguation page; opensearch
	// surfaces "Dune (novel)" which has Frank Herbert in the extract.
	startFakeWikipedia(t, fakeWiki{
		summaryByTitle: map[string]string{
			"Dune": `{
				"type": "disambiguation",
				"title": "Dune",
				"description": "Topics referred to by the same term",
				"extract": "Dune may refer to..."
			}`,
			"Dune_(novel)": `{
				"type": "standard",
				"title": "Dune (novel)",
				"description": "1965 novel by Frank Herbert",
				"extract": "Dune is a 1965 epic science fiction novel by American author Frank Herbert."
			}`,
		},
		openSearchHits: []string{"Dune (novel)", "Dune (franchise)"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchWikipediaSummary(ctx, "Dune", []string{"Frank Herbert"})
	if err != nil {
		t.Fatalf("FetchWikipediaSummary: %v", err)
	}
	if !strings.Contains(got, "Frank Herbert") {
		t.Errorf("expected the (novel) article extract; got %q", got)
	}
}

func TestFetchWikipediaSummary_OpenSearchWalksToFirstAuthorMatch(t *testing.T) {
	// First two opensearch hits are irrelevant topics; the third
	// matches the author and wins.
	startFakeWikipedia(t, fakeWiki{
		summaryByTitle: map[string]string{
			"Foundation": `{
				"type": "disambiguation",
				"title": "Foundation",
				"extract": "Foundation may refer to..."
			}`,
			"Foundation_(building)": `{
				"type": "standard",
				"title": "Foundation (building)",
				"description": "Structural part of a building",
				"extract": "A foundation is the lowest load-bearing part of a building."
			}`,
			"Foundation_(charity)": `{
				"type": "standard",
				"title": "Foundation (charity)",
				"description": "Type of nonprofit organization",
				"extract": "A foundation is a category of nonprofit organization."
			}`,
			"Foundation_(Asimov_novel)": `{
				"type": "standard",
				"title": "Foundation (Asimov novel)",
				"description": "1951 novel by Isaac Asimov",
				"extract": "Foundation is a science fiction novel by American writer Isaac Asimov."
			}`,
		},
		openSearchHits: []string{
			"Foundation (building)",
			"Foundation (charity)",
			"Foundation (Asimov novel)",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchWikipediaSummary(ctx, "Foundation", []string{"Isaac Asimov"})
	if err != nil {
		t.Fatalf("FetchWikipediaSummary: %v", err)
	}
	if !strings.Contains(got, "Isaac Asimov") {
		t.Errorf("expected the Asimov novel extract; got %q", got)
	}
}

func TestFetchWikipediaSummary_NotFoundReturnsEmpty(t *testing.T) {
	startFakeWikipedia(t, fakeWiki{
		summaryByTitle: map[string]string{},
		openSearchHits: []string{},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchWikipediaSummary(ctx, "Imaginary Title 12345", []string{"Nobody Author"})
	if err != nil {
		t.Fatalf("FetchWikipediaSummary: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty on 404; got %q", got)
	}
}

func TestFetchWikipediaSummary_EmptyTitleReturnsEmpty(t *testing.T) {
	// Should not even hit the API; defensive guard.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchWikipediaSummary(ctx, "", []string{"Someone"})
	if err != nil {
		t.Fatalf("FetchWikipediaSummary(\"\"): %v", err)
	}
	if got != "" {
		t.Errorf("expected empty for empty title; got %q", got)
	}
}

func TestFetchWikipediaSummary_NoAuthorsNoFallback(t *testing.T) {
	// Direct title misses (disambiguation), and we have no authors
	// for opensearch to disambiguate -- must give up rather than guess.
	startFakeWikipedia(t, fakeWiki{
		summaryByTitle: map[string]string{
			"Foundation": `{"type": "disambiguation", "extract": "..."}`,
		},
		openSearchHits: []string{"Foundation (building)"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := FetchWikipediaSummary(ctx, "Foundation", nil)
	if err != nil {
		t.Fatalf("FetchWikipediaSummary: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty when disambiguation + no authors; got %q", got)
	}
}

func TestExtractMatchesAuthor(t *testing.T) {
	cases := []struct {
		name    string
		sum     wikipediaSummary
		authors []string
		want    bool
	}{
		{
			name:    "extract contains author",
			sum:     wikipediaSummary{Extract: "A novel by Frank Herbert."},
			authors: []string{"Frank Herbert"},
			want:    true,
		},
		{
			name:    "description contains author when extract does not",
			sum:     wikipediaSummary{Description: "1965 novel by Frank Herbert", Extract: "Set on Arrakis..."},
			authors: []string{"Frank Herbert"},
			want:    true,
		},
		{
			name:    "case-insensitive match",
			sum:     wikipediaSummary{Extract: "A novel by FRANK HERBERT."},
			authors: []string{"frank herbert"},
			want:    true,
		},
		{
			name:    "any author in slice can match (second wins)",
			sum:     wikipediaSummary{Extract: "Co-written by Neil Gaiman and Terry Pratchett."},
			authors: []string{"Someone Else", "Terry Pratchett"},
			want:    true,
		},
		{
			name:    "partial last-name in extract does not match full-name author",
			sum:     wikipediaSummary{Extract: "Co-written by Pratchett."},
			authors: []string{"Terry Pratchett"},
			want:    false, // haystack lacks "terry pratchett" -- only "pratchett"
		},
		{
			name:    "no match",
			sum:     wikipediaSummary{Extract: "A landform composed of sand."},
			authors: []string{"Frank Herbert"},
			want:    false,
		},
		{
			name:    "empty authors slice means guard skipped (returns true)",
			sum:     wikipediaSummary{Extract: "Anything."},
			authors: nil,
			want:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractMatchesAuthor(&tc.sum, tc.authors)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
