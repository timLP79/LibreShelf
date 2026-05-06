// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package safezip

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type zipEntry struct {
	name   string
	body   []byte
	mode   os.FileMode
	method uint16
}

func makeZip(t *testing.T, dir string, entries []zipEntry) string {
	t.Helper()
	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for _, e := range entries {
		fh := &zip.FileHeader{Name: e.name, Method: e.method}
		if e.mode != 0 {
			fh.SetMode(e.mode)
		}
		ww, err := w.CreateHeader(fh)
		if err != nil {
			t.Fatalf("create header %q: %v", e.name, err)
		}
		if e.body != nil {
			if _, err := ww.Write(e.body); err != nil {
				t.Fatalf("write %q: %v", e.name, err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return zipPath
}

// makeZipBomb writes a Deflate ZIP and patches the central directory header
// to claim a smaller UncompressedSize than the body actually decompresses to.
// Reader uses the CD value (so f.UncompressedSize64 reads the lie) while the
// compressed stream still produces all the real bytes through the decompressor,
// which is what the LimitReader+1 check in extractEntry is designed to catch.
func makeZipBomb(t *testing.T, dir string, body []byte, claimed uint32) string {
	t.Helper()
	zipPath := filepath.Join(dir, "bomb.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	w := zip.NewWriter(f)
	ww, err := w.CreateHeader(&zip.FileHeader{Name: "bomb.txt", Method: zip.Deflate})
	if err != nil {
		t.Fatalf("create header: %v", err)
	}
	if _, err := ww.Write(body); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	data, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	// Locate the CD via EOCD (last 22 bytes; the Go writer adds no archive
	// comment). Then locate the data descriptor by walking past the local
	// file header and the compressed payload. Both must be patched: Go's
	// Reader cross-checks the data descriptor's UncompressedSize against
	// the CD value at end-of-stream and rejects the archive on mismatch.
	eocdOff := len(data) - 22
	if binary.LittleEndian.Uint32(data[eocdOff:eocdOff+4]) != 0x06054b50 {
		t.Fatalf("EOCD not at expected position %d", eocdOff)
	}
	cdOff := int(binary.LittleEndian.Uint32(data[eocdOff+16 : eocdOff+20]))
	if binary.LittleEndian.Uint32(data[cdOff:cdOff+4]) != 0x02014b50 {
		t.Fatalf("CD signature not at offset %d", cdOff)
	}
	nameLen := int(binary.LittleEndian.Uint16(data[26:28]))
	extraLen := int(binary.LittleEndian.Uint16(data[28:30]))
	compressedSize := int(binary.LittleEndian.Uint32(data[cdOff+20 : cdOff+24]))
	ddOff := 30 + nameLen + extraLen + compressedSize
	if binary.LittleEndian.Uint32(data[ddOff:ddOff+4]) != 0x08074b50 {
		t.Fatalf("data descriptor signature not at offset %d", ddOff)
	}
	binary.LittleEndian.PutUint32(data[ddOff+12:ddOff+16], claimed)
	binary.LittleEndian.PutUint32(data[cdOff+24:cdOff+28], claimed)
	if err := os.WriteFile(zipPath, data, 0o644); err != nil {
		t.Fatalf("rewrite zip: %v", err)
	}
	return zipPath
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return n
}

func TestSafeExtract_Happy_SimpleFile(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	zipPath := makeZip(t, src, []zipEntry{{name: "hello.txt", body: []byte("hello world")}})
	if err := SafeExtract(zipPath, dst); err != nil {
		t.Fatalf("SafeExtract: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("body = %q, want %q", got, "hello world")
	}
}

func TestSafeExtract_Happy_Empty(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	zipPath := makeZip(t, src, nil)
	if err := SafeExtract(zipPath, dst); err != nil {
		t.Fatalf("SafeExtract: %v", err)
	}
	if got := countFiles(t, dst); got != 0 {
		t.Errorf("dst file count = %d, want 0", got)
	}
}

func TestSafeExtract_Happy_NestedSafe(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	zipPath := makeZip(t, src, []zipEntry{{name: "covers/sub/img.dat", body: []byte("body")}})
	if err := SafeExtract(zipPath, dst); err != nil {
		t.Fatalf("SafeExtract: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "covers", "sub", "img.dat"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(got) != "body" {
		t.Errorf("body = %q, want %q", got, "body")
	}
}

func TestSafeExtract_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		entry   zipEntry
		wantErr error
	}{
		{"zipslip parent", zipEntry{name: "../evil.txt", body: []byte("x")}, ErrZipSlip},
		{"zipslip nested escape", zipEntry{name: "subdir/../../evil.txt", body: []byte("x")}, ErrZipSlip},
		{"backslash", zipEntry{name: `..\evil.txt`, body: []byte("x")}, ErrZipSlip},
		{"absolute", zipEntry{name: "/etc/passwd", body: []byte("x")}, ErrAbsolutePath},
		{"symlink", zipEntry{name: "link", body: []byte("/etc"), mode: 0o777 | os.ModeSymlink}, ErrSymlink},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, dst := t.TempDir(), t.TempDir()
			zipPath := makeZip(t, src, []zipEntry{tc.entry})
			err := SafeExtract(zipPath, dst)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want errors.Is(%v)", err, tc.wantErr)
			}
			if got := countFiles(t, dst); got != 0 {
				t.Errorf("dst should have 0 files after rejection, got %d", got)
			}
		})
	}
}

func TestSafeExtract_PerFileLimit(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	zipPath := makeZip(t, src, []zipEntry{{name: "big.bin", body: make([]byte, 1024)}})
	err := SafeExtractWithLimits(zipPath, dst, Limits{MaxFileSize: 512})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want errors.Is(ErrTooLarge)", err)
	}
}

func TestSafeExtract_TotalLimit(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	zipPath := makeZip(t, src, []zipEntry{
		{name: "a.bin", body: make([]byte, 400)},
		{name: "b.bin", body: make([]byte, 400)},
		{name: "c.bin", body: make([]byte, 400)},
	})
	err := SafeExtractWithLimits(zipPath, dst, Limits{MaxTotalSize: 1000})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want errors.Is(ErrTooLarge)", err)
	}
}

func TestSafeExtract_ZeroLimitsUnlimited(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	zipPath := makeZip(t, src, []zipEntry{{name: "big.bin", body: make([]byte, 1<<20)}})
	if err := SafeExtractWithLimits(zipPath, dst, Limits{}); err != nil {
		t.Fatalf("SafeExtractWithLimits with zero limits: %v", err)
	}
}

func TestSafeExtract_Atomicity_NoPartialWrite(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	entries := make([]zipEntry, 10)
	for i := range entries {
		entries[i] = zipEntry{name: fmt.Sprintf("ok/file%d.txt", i), body: []byte("ok")}
	}
	entries[5] = zipEntry{name: "../evil.txt", body: []byte("x")}
	zipPath := makeZip(t, src, entries)
	err := SafeExtract(zipPath, dst)
	if !errors.Is(err, ErrZipSlip) {
		t.Fatalf("err = %v, want errors.Is(ErrZipSlip)", err)
	}
	if got := countFiles(t, dst); got != 0 {
		t.Errorf("dst file count = %d, want 0 (no partial extraction)", got)
	}
}

func TestSafeExtract_EmptyName(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	zipPath := makeZip(t, src, []zipEntry{{name: "", body: []byte("x")}})
	if err := SafeExtract(zipPath, dst); err == nil {
		t.Errorf("SafeExtract on empty-name entry should error, got nil")
	}
}

func TestSafeExtract_DotName(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	zipPath := makeZip(t, src, []zipEntry{{name: ".", body: []byte("x")}})
	if err := SafeExtract(zipPath, dst); err == nil {
		t.Errorf("SafeExtract on '.' entry should error, got nil")
	}
}

func TestSafeExtract_ZipBomb(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	body := bytes.Repeat([]byte{'A'}, 1000)
	zipPath := makeZipBomb(t, src, body, 50)
	err := SafeExtractWithLimits(zipPath, dst, Limits{})
	if err == nil {
		t.Fatal("SafeExtract on zip bomb should return error, got nil")
	}
	// Either the stdlib's checksumReader rejects (archive/zip.ErrFormat,
	// the path that fires on Go 1.21+) or our LimitReader+1 rejects
	// (ErrTooLarge). Both indicate detection; what matters is the bomb's
	// full payload was not written to dst.
	if info, statErr := os.Stat(filepath.Join(dst, "bomb.txt")); statErr == nil && info.Size() >= int64(len(body)) {
		t.Errorf("bomb.txt size = %d, expected detection before full extraction", info.Size())
	}
}
