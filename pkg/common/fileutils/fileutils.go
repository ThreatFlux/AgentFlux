// Package fileutils provides common file and path handling functions.
package fileutils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// IsHiddenFile checks if a file is hidden.
// On Unix-like systems, this checks if the filename starts with a dot.
// On Windows, it checks the file attributes.
func IsHiddenFile(path string) bool {
	// Get the base name of the file
	filename := filepath.Base(path)
	
	// Check if the filename starts with a dot (Unix-like systems)
	if strings.HasPrefix(filename, ".") && filename != "." && filename != ".." {
		return true
	}
	
	// On Windows, check file attributes (not implemented here)
	// This would require syscall functionality specific to Windows
	
	return false
}

// IsSymlink checks if a file is a symbolic link.
func IsSymlink(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// IsExecutable checks if a file is executable.
func IsExecutable(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	return info.Mode()&0111 != 0
}

// SafeReadFile reads a file with size limits to prevent memory exhaustion.
// If maxSize is 0 or negative, no size limit is applied.
// If the file is larger than maxSize, it returns only the first maxSize bytes.
func SafeReadFile(path string, maxSize int64) ([]byte, error) {
	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	// If no size limit or negative, read the entire file
	if maxSize <= 0 {
		return io.ReadAll(file)
	}
	
	// Otherwise, limit the reading to maxSize bytes
	return io.ReadAll(io.LimitReader(file, maxSize))
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) error {
	// Validate input paths
	if src == "" || dst == "" {
		return fmt.Errorf("source or destination path is empty")
	}
	
	// Check if source exists and is a regular file
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}
	
	// Check if destination directory exists
	dstDir := filepath.Dir(dst)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		return fmt.Errorf("destination directory %s does not exist", dstDir)
	}
	
	// Open the source file
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	
	// Create the destination file
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	
	// Copy the contents
	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}
	
	// Sync data to storage
	if err = destination.Sync(); err != nil {
		return err
	}
	
	// Get source file info for permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	
	// Set the same permissions
	return os.Chmod(dst, sourceInfo.Mode())
}

// CreateDirIfNotExist creates a directory if it doesn't exist.
func CreateDirIfNotExist(dir string) error {
	if dir == "" {
		return fmt.Errorf("directory path is empty")
	}
	
	info, err := os.Stat(dir)
	if err == nil {
		// Path exists - make sure it's a directory
		if !info.IsDir() {
			return fmt.Errorf("%s exists but is not a directory", dir)
		}
		return nil
	}
	
	if !os.IsNotExist(err) {
		// Some other error
		return err
	}
	
	// Directory doesn't exist, create it with all parents
	return os.MkdirAll(dir, 0755)
}

// PathExists checks if a path exists.
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// FileExtension returns the extension of a file without the dot.
func FileExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	return ext[1:] // Remove the leading dot
}

// ReadFileLines reads a file and returns its lines as a string slice
func ReadFileLines(path string) ([]string, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Handle empty file
	if len(data) == 0 {
		return []string{}, nil
	}

	// Normalize line endings and split
	content := string(data)
	content = strings.ReplaceAll(content, "\r\n", "\n") // Convert Windows to Unix
	return strings.Split(content, "\n"), nil
}

// DirIsEmpty checks if a directory is empty
func DirIsEmpty(path string) (bool, error) {
	// Open the directory
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Read directory entries - note we only need to know if any entries exist,
	// so we don't store them in a variable
	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// If we got here, there's at least one entry
	return false, nil
}

// CompareFiles compares two files and returns true if they have the same content
func CompareFiles(file1, file2 string) (bool, error) {
	// Get file info for both files
	info1, err := os.Stat(file1)
	if err != nil {
		return false, err
	}
	info2, err := os.Stat(file2)
	if err != nil {
		return false, err
	}

	// Check if both are regular files
	if !info1.Mode().IsRegular() || !info2.Mode().IsRegular() {
		return false, fmt.Errorf("not regular files")
	}

	// Quick check for file size
	if info1.Size() != info2.Size() {
		return false, nil
	}

	// Compare content
	f1, err := os.Open(file1)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(file2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	// Use a buffer for comparison to avoid loading entire files into memory
	const bufferSize = 64 * 1024 // 64KB
	buf1 := make([]byte, bufferSize)
	buf2 := make([]byte, bufferSize)

	for {
		n1, err1 := f1.Read(buf1)
		n2, err2 := f2.Read(buf2)

		// Check for read errors
		if err1 != nil && err1 != io.EOF {
			return false, err1
		}
		if err2 != nil && err2 != io.EOF {
			return false, err2
		}

		// Check for different read sizes
		if n1 != n2 {
			return false, nil
		}

		// End of both files
		if err1 == io.EOF && err2 == io.EOF {
			break
		}

		// Compare chunks
		if !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return false, nil
		}
	}

	return true, nil
}

// CreateTempFile is a helper for creating temporary files in tests
func CreateTempFile(t testing.TB, dir, prefix, suffix string, content []byte) string {
	if t == nil {
		panic("CreateTempFile: testing.TB is nil")
	}
	
	// Create a temporary file
	file, err := os.CreateTemp(dir, prefix+"*"+suffix)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer file.Close()

	// Write content
	if _, err := file.Write(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	return file.Name()
}
