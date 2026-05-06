// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package safezip

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrZipSlip      = errors.New("entry path escapes destination")
	ErrSymlink      = errors.New("symlinks not permitted")
	ErrAbsolutePath = errors.New("absolute path not permitted")
	ErrTooLarge     = errors.New("entry or archive exceeds size limit")
)

type Limits struct {
	MaxFileSize  int64
	MaxTotalSize int64
}

var DefaultLimits = Limits{
	MaxFileSize:  100 << 20,
	MaxTotalSize: 500 << 20,
}

func SafeExtract(zipPath, destDir string) error {
	return SafeExtractWithLimits(zipPath, destDir, DefaultLimits)
}

func SafeExtractWithLimits(zipPath, destDir string, limits Limits) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	var totalSize int64
	for _, f := range r.File {
		if err := validateEntry(f, absDest, limits); err != nil {
			return err
		}
		if !f.FileInfo().IsDir() {
			totalSize += int64(f.UncompressedSize64)
			if limits.MaxTotalSize > 0 && totalSize > limits.MaxTotalSize {
				return fmt.Errorf("%w: archive total exceeds limit", ErrTooLarge)
			}
		}
	}

	for _, f := range r.File {
		if err := extractEntry(f, absDest); err != nil {
			return err
		}
	}
	return nil
}

func validateEntry(f *zip.File, absDest string, limits Limits) error {
	if f.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %q", ErrSymlink, f.Name)
	}
	if strings.Contains(f.Name, "\\") {
		return fmt.Errorf("%w: %q (backslash not permitted)", ErrZipSlip, f.Name)
	}
	if filepath.IsAbs(f.Name) {
		return fmt.Errorf("%w: %q", ErrAbsolutePath, f.Name)
	}
	target := filepath.Join(absDest, f.Name)
	rel, err := filepath.Rel(absDest, target)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrZipSlip, f.Name)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: %q", ErrZipSlip, f.Name)
	}
	if !f.FileInfo().IsDir() && limits.MaxFileSize > 0 {
		if int64(f.UncompressedSize64) > limits.MaxFileSize {
			return fmt.Errorf("%w: %q", ErrTooLarge, f.Name)
		}
	}
	return nil
}

func extractEntry(f *zip.File, absDest string) error {
	target := filepath.Join(absDest, f.Name)
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	limited := io.LimitReader(rc, int64(f.UncompressedSize64)+1)
	n, err := io.Copy(out, limited)
	if err != nil {
		return err
	}
	if n > int64(f.UncompressedSize64) {
		return fmt.Errorf("%w: %q (zip bomb)", ErrTooLarge, f.Name)
	}
	return nil
}
