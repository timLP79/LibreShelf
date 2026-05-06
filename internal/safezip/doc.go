// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

// Package safezip provides ZIP extraction with protection against
// path-traversal attacks (Zip Slip), absolute-path entries, symlinks,
// and zip-bomb attacks. The lying-header check in extractEntry is
// defense-in-depth; Go 1.21+ archive/zip already rejects decompression
// that exceeds the declared UncompressedSize64.
//
// All extractions use two-pass validation: every entry in the archive
// is validated before any file is written to disk. A malicious entry
// at any position causes the entire extraction to abort with no
// partial state on disk.
//
// Defaults via DefaultLimits cap individual entries at 100 MB
// uncompressed and the archive total at 500 MB. Use
// SafeExtractWithLimits to override.
package safezip
