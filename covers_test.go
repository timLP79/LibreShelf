// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtensionForImageMime(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"image/gif", ""},
		{"text/html", ""},
		{"", ""},
		{"IMAGE/JPEG", ""}, // case-sensitive
	}
	for _, tc := range cases {
		if got := extensionForImageMime(tc.in); got != tc.want {
			t.Errorf("extensionForImageMime(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRandomCoverFilename(t *testing.T) {
	got, err := randomCoverFilename(".jpg")
	if err != nil {
		t.Fatalf("randomCoverFilename: %v", err)
	}
	if !strings.HasSuffix(got, ".jpg") {
		t.Errorf("filename = %q, expected .jpg suffix", got)
	}
	// 32 hex chars (16 bytes) + ".jpg" (4 chars) = 36
	if len(got) != 36 {
		t.Errorf("filename = %q, expected length 36, got %d", got, len(got))
	}
	// Should be unique across calls.
	other, _ := randomCoverFilename(".jpg")
	if other == got {
		t.Errorf("two calls produced the same filename: %q", got)
	}
}

// realJPEG returns a tiny but byte-valid JPEG image so http.DetectContentType
// reports image/jpeg. A short fake byte slice would not pass the sniffer.
func realJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg encode: %v", err)
	}
	return buf.Bytes()
}

func realPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func TestSaveCoverFromURL_Happy(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DATA_DIR", tmp)

	body := realJPEG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	filename, err := SaveCoverFromURL(srv.URL + "/cover.jpg")
	if err != nil {
		t.Fatalf("SaveCoverFromURL: %v", err)
	}
	if !strings.HasSuffix(filename, ".jpg") {
		t.Errorf("filename = %q, expected .jpg suffix", filename)
	}
	full := filepath.Join(tmp, "covers", filename)
	got, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read saved cover: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("saved bytes != source bytes")
	}
}

func TestSaveCoverFromURL_HTTPError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DATA_DIR", tmp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	_, err := SaveCoverFromURL(srv.URL + "/missing.jpg")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v, want non-nil mentioning 404", err)
	}
}

func TestSaveCoverFromURL_TooLarge(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DATA_DIR", tmp)
	// Body that exceeds the 2MB limit.
	big := bytes.Repeat([]byte{0xff}, coverMaxBytes+10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(big)
	}))
	t.Cleanup(srv.Close)

	_, err := SaveCoverFromURL(srv.URL + "/big.jpg")
	if !errors.Is(err, ErrCoverTooLarge) {
		t.Errorf("err = %v, want ErrCoverTooLarge", err)
	}
}

func TestSaveCoverFromURL_BadMime(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DATA_DIR", tmp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	t.Cleanup(srv.Close)

	_, err := SaveCoverFromURL(srv.URL + "/fake.jpg")
	if !errors.Is(err, ErrCoverBadMimeType) {
		t.Errorf("err = %v, want ErrCoverBadMimeType", err)
	}
}

func TestSaveCoverFromURL_PNGContentType(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DATA_DIR", tmp)
	body := realPNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	filename, err := SaveCoverFromURL(srv.URL + "/cover.png")
	if err != nil {
		t.Fatalf("SaveCoverFromURL: %v", err)
	}
	if !strings.HasSuffix(filename, ".png") {
		t.Errorf("filename = %q, expected .png suffix", filename)
	}
}
