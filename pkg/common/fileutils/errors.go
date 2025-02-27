package fileutils

import "errors"

// Common errors for file operations
var (
	ErrFileTooLarge = errors.New("file too large")
	ErrInvalidPath  = errors.New("invalid file path")
	ErrAccessDenied = errors.New("access denied")
)
