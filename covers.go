// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	coverMaxBytes        = 2 * 1024 * 1024
	coverDownloadTimeout = 10 * time.Second
)

var (
	ErrCoverTooLarge     = errors.New("cover: file exceeds 2MB limit")
	ErrCoverBadExtension = errors.New("cover: extension must be jpg, jpeg, png, or webp")
	ErrCoverBadMimeType  = errors.New("cover: content does not match a supported image type")
)

func coversDir() string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "data"
	}
	return filepath.Join(dir, "covers")
}

func randomCoverFilename(ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + ext, nil
}

func extensionForImageMime(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}
	return ""
}

func SaveUploadedCover(fh *multipart.FileHeader) (string, error) {
	if fh.Size > coverMaxBytes {
		return "", ErrCoverTooLarge
	}

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	expectedMime := ""
	switch ext {
	case ".jpg":
		expectedMime = "image/jpeg"
	case ".png":
		expectedMime = "image/png"
	case ".webp":
		expectedMime = "image/webp"
	default:
		return "", ErrCoverBadExtension
	}

	file, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	detected := http.DetectContentType(head[:n])
	if detected != expectedMime {
		return "", ErrCoverBadMimeType
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return writeCoverStream(file, ext)
}

// SaveCoverFromURLGated is the offline-aware entry point for any caller
// that has a DatabaseManager. Returns ErrExternalDisabled without making
// any HTTP attempt when external calls are blocked.
//
// Tests that need to drive the HTTP path against httptest.NewServer
// should keep calling SaveCoverFromURL directly.
func SaveCoverFromURLGated(dm *DatabaseManager, url string) (string, error) {
	if !IsExternalAllowed(dm) {
		return "", ErrExternalDisabled
	}
	return SaveCoverFromURL(url)
}

func SaveCoverFromURL(url string) (string, error) {
	client := &http.Client{Timeout: coverDownloadTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover download: upstream returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, coverMaxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > coverMaxBytes {
		return "", ErrCoverTooLarge
	}

	detected := http.DetectContentType(data)
	ext := extensionForImageMime(detected)
	if ext == "" {
		return "", ErrCoverBadMimeType
	}

	filename, err := randomCoverFilename(ext)
	if err != nil {
		return "", err
	}
	dir := coversDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	full := filepath.Join(dir, filename)
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", err
	}
	return filename, nil
}

func writeCoverStream(src io.Reader, ext string) (string, error) {
	filename, err := randomCoverFilename(ext)
	if err != nil {
		return "", err
	}
	dir := coversDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	full := filepath.Join(dir, filename)
	dst, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, io.LimitReader(src, coverMaxBytes)); err != nil {
		os.Remove(full)
		return "", err
	}
	return filename, nil
}
