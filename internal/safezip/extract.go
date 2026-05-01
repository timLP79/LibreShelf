package safezip

import "errors"

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
