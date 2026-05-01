// Package safezip provides ZIP extraction with protection against
// path-traversal attacks (Zip Slip), absolute-path entries, symlinks,
// and zip-bomb attacks via lying-header defense.
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
