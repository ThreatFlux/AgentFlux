package fileutils

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsHiddenFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Regular file",
			path:     "/path/to/regular.txt",
			expected: false,
		},
		{
			name:     "Hidden file",
			path:     "/path/to/.hidden",
			expected: true,
		},
		{
			name:     "File with dot in name",
			path:     "/path/to/file.txt",
			expected: false,
		},
		{
			name:     "Hidden file in hidden directory",
			path:     "/path/to/.hidden/file.txt",
			expected: false,
		},
		{
			name:     "Empty path",
			path:     "",
			expected: false,
		},
		{
			name:     "Path with just dot",
			path:     ".",
			expected: false,
		},
		{
			name:     "Path with just dot-dot",
			path:     "..",
			expected: false,
		},
		{
			name:     "Root path",
			path:     "/",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsHiddenFile(tc.path)
			if result != tc.expected {
				t.Errorf("IsHiddenFile(%s) = %v, want %v", tc.path, result, tc.expected)
			}
		})
	}
}

func TestIsSymlink(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "symlink_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a regular file
	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a symlink if platform supports it
	symlinkFile := filepath.Join(tmpDir, "symlink.txt")
	os.Remove(symlinkFile) // Remove if exists
	err = os.Symlink(regularFile, symlinkFile)
	if err != nil {
		t.Logf("Symlink creation not supported on this platform, skipping symlink tests: %v", err)
	} else {
		// Test symlink
		regularInfo, err := os.Lstat(regularFile)
		if err != nil {
			t.Fatalf("Failed to get file info: %v", err)
		}
		
		symlinkInfo, err := os.Lstat(symlinkFile)
		if err != nil {
			t.Fatalf("Failed to get symlink info: %v", err)
		}
		
		if IsSymlink(regularInfo) {
			t.Errorf("Expected regular file not to be a symlink")
		}
		
		if !IsSymlink(symlinkInfo) {
			t.Errorf("Expected symlink to be detected as a symlink")
		}
	}

	// Test with directory
	dirInfo, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get directory info: %v", err)
	}
	
	if IsSymlink(dirInfo) {
		t.Errorf("Expected directory not to be detected as a symlink")
	}

	// Test with nil FileInfo (should not panic)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("IsSymlink(nil) panicked: %v", r)
		}
	}()
	
	// This should not panic but return false
	if IsSymlink(nil) {
		t.Errorf("Expected IsSymlink(nil) to return false")
	}
}

func TestIsExecutable(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "exec_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a non-executable file
	nonExecFile := filepath.Join(tmpDir, "nonexec.txt")
	if err := os.WriteFile(nonExecFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create non-executable file: %v", err)
	}

	// Create an executable file
	execFile := filepath.Join(tmpDir, "exec.sh")
	if err := os.WriteFile(execFile, []byte("#!/bin/sh\necho 'test'"), 0755); err != nil {
		t.Fatalf("Failed to create executable file: %v", err)
	}

	// Test non-executable
	nonExecInfo, err := os.Stat(nonExecFile)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}
	
	if IsExecutable(nonExecInfo) {
		t.Errorf("Expected non-executable file to return false")
	}
	
	// Test executable
	execInfo, err := os.Stat(execFile)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}
	
	// On Windows, executable bit doesn't work the same way
	// so we might not see the executable flag as expected
	if runtime.GOOS != "windows" && !IsExecutable(execInfo) {
		t.Errorf("Expected executable file to return true")
	}
	
	// Test directory (may or may not be executable depending on OS)
	dirInfo, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get directory info: %v", err)
	}
	
	// Just call the function to ensure coverage, don't check result
	_ = IsExecutable(dirInfo)
	
	// Test with nil FileInfo (should not panic)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("IsExecutable(nil) panicked: %v", r)
		}
	}()
	
	// This should not panic but return false
	if IsExecutable(nil) {
		t.Errorf("Expected IsExecutable(nil) to return false")
	}
}

func TestFileExtension(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Regular file with extension",
			path:     "/path/to/file.txt",
			expected: "txt",
		},
		{
			name:     "File without extension",
			path:     "/path/to/file",
			expected: "",
		},
		{
			name:     "Hidden file with extension",
			path:     "/path/to/.hidden.txt",
			expected: "txt",
		},
		{
			name:     "Multiple extensions",
			path:     "/path/to/file.tar.gz",
			expected: "gz",
		},
		{
			name:     "Empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "Path with just a dot",
			path:     ".",
			expected: "",
		},
		{
			name:     "Path ending with a dot",
			path:     "/path/to/file.",
			expected: "",
		},
		{
			name:     "Double dots",
			path:     "..",
			expected: "",
		},
		{
			name:     "Path with dots in directory names",
			path:     "/path/with.dots/in.dirs/file.txt",
			expected: "txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FileExtension(tc.path)
			if result != tc.expected {
				t.Errorf("FileExtension(%s) = %v, want %v", tc.path, result, tc.expected)
			}
		})
	}
}

func TestSafeReadFile(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "fileutils_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	testFilePath := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create empty test file
	emptyFilePath := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFilePath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty test file: %v", err)
	}

	// Create a large test file
	largeFilePath := filepath.Join(tmpDir, "large.bin")
	largeContent := make([]byte, 1024*16) // 16KB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(largeFilePath, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	// Create a directory that should fail to read
	dirPath := filepath.Join(tmpDir, "directory")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		maxSize     int64
		expectError bool
		expectData  []byte
		errorType   error // Type of error expected, nil for any error
	}{
		{
			name:        "Read with sufficient max size",
			path:        testFilePath,
			maxSize:     100,
			expectError: false,
			expectData:  testContent,
		},
		{
			name:        "Read with exact max size",
			path:        testFilePath,
			maxSize:     int64(len(testContent)),
			expectError: false,
			expectData:  testContent,
		},
		{
			name:        "Read with insufficient max size",
			path:        testFilePath,
			maxSize:     5, // Less than the content length
			expectError: false,
			expectData:  testContent[:5],
		},
		{
			name:        "Read non-existent file",
			path:        filepath.Join(tmpDir, "nonexistent.txt"),
			maxSize:     100,
			expectError: true,
			expectData:  nil,
			errorType:   &os.PathError{},
		},
		{
			name:        "Read with unlimited max size",
			path:        testFilePath,
			maxSize:     0,
			expectError: false,
			expectData:  testContent,
		},
		{
			name:        "Read with negative max size (unlimited)",
			path:        testFilePath,
			maxSize:     -1,
			expectError: false,
			expectData:  testContent,
		},
		{
			name:        "Read empty file",
			path:        emptyFilePath,
			maxSize:     100,
			expectError: false,
			expectData:  []byte{},
		},
		{
			name:        "Read large file with limit",
			path:        largeFilePath,
			maxSize:     1024, // 1KB limit
			expectError: false,
			expectData:  largeContent[:1024],
		},
		{
			name:        "Read directory should fail",
			path:        dirPath,
			maxSize:     100,
			expectError: true,
			expectData:  nil,
		},
		{
			name:        "Read with zero path",
			path:        "",
			maxSize:     100,
			expectError: true,
			expectData:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := SafeReadFile(tc.path, tc.maxSize)

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tc.expectError && tc.errorType != nil && err != nil {
				// Check error type if specified
				var pathErr *os.PathError
				if errors.As(err, &pathErr) {
					// Error type matches expected type
				} else {
					t.Errorf("Expected error of type %T, got %T: %v", tc.errorType, err, err)
				}
			}

			if !tc.expectError && tc.expectData != nil {
				if !equalByteSlices(data, tc.expectData) {
					if len(data) != len(tc.expectData) {
						t.Errorf("Expected data length %d, got %d", len(tc.expectData), len(data))
					} else {
						// Find first difference
						for i := range data {
							if data[i] != tc.expectData[i] {
								t.Errorf("Data difference at position %d: expected 0x%02x, got 0x%02x", 
									i, tc.expectData[i], data[i])
								break
							}
						}
					}
				}
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "copyfile_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	srcContent := []byte("source file content")
	if err := os.WriteFile(srcPath, srcContent, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create empty source file
	emptySrcPath := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptySrcPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty source file: %v", err)
	}

	// Create directory
	dirPath := filepath.Join(tmpDir, "directory")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Test copy to new destination
	dstPath := filepath.Join(tmpDir, "destination.txt")
	err = CopyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("Failed to copy file: %v", err)
	}

	// Check destination content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if !equalByteSlices(srcContent, dstContent) {
		t.Errorf("Destination content does not match source content")
	}

	// Check permissions
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		t.Fatalf("Failed to get source file info: %v", err)
	}

	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("Failed to get destination file info: %v", err)
	}

	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Errorf("Destination mode %v does not match source mode %v", dstInfo.Mode().Perm(), srcInfo.Mode().Perm())
	}

	// Test copy to nonexistent directory
	invalidPath := filepath.Join(tmpDir, "nonexistent", "file.txt")
	err = CopyFile(srcPath, invalidPath)
	if err == nil {
		t.Errorf("Expected error when copying to nonexistent directory, got none")
	}

	// Test copy from nonexistent source
	nonexistentSrc := filepath.Join(tmpDir, "nonexistent.txt")
	err = CopyFile(nonexistentSrc, dstPath)
	if err == nil {
		t.Errorf("Expected error when copying from nonexistent source, got none")
	}

	// Test copy to directory (should fail)
	err = CopyFile(srcPath, dirPath)
	if err == nil {
		t.Errorf("Expected error when copying to a directory, got none")
	}

	// Test copy from directory (should fail)
	dirDstPath := filepath.Join(tmpDir, "dirCopy")
	err = CopyFile(dirPath, dirDstPath)
	if err == nil {
		t.Errorf("Expected error when copying from a directory, got none")
	}

	// Test copy from empty file
	emptyDstPath := filepath.Join(tmpDir, "emptyDst.txt")
	err = CopyFile(emptySrcPath, emptyDstPath)
	if err != nil {
		t.Errorf("Failed to copy empty file: %v", err)
	}

	// Test copy to existing file (should overwrite)
	existingDstPath := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(existingDstPath, []byte("existing content"), 0644); err != nil {
		t.Fatalf("Failed to create existing destination file: %v", err)
	}

	err = CopyFile(srcPath, existingDstPath)
	if err != nil {
		t.Errorf("Failed to copy to existing file: %v", err)
	}

	// Check content was overwritten
	overwrittenContent, err := os.ReadFile(existingDstPath)
	if err != nil {
		t.Fatalf("Failed to read overwritten file: %v", err)
	}

	if !equalByteSlices(srcContent, overwrittenContent) {
		t.Errorf("Overwritten content does not match source content")
	}

	// Test with empty paths
	err = CopyFile("", dstPath)
	if err == nil {
		t.Errorf("Expected error when copying from empty path, got none")
	}

	err = CopyFile(srcPath, "")
	if err == nil {
		t.Errorf("Expected error when copying to empty path, got none")
	}
}

func TestCreateDirIfNotExist(t *testing.T) {
	// Create temporary root directory
	tmpDir, err := os.MkdirTemp("", "createdir_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test creating a new directory
	newDir := filepath.Join(tmpDir, "newdir")
	err = CreateDirIfNotExist(newDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Check if directory exists
	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", newDir)
	}

	// Test creating an existing directory (should not error)
	err = CreateDirIfNotExist(newDir)
	if err != nil {
		t.Errorf("Unexpected error when creating existing directory: %v", err)
	}

	// Test creating nested directories
	nestedDir := filepath.Join(tmpDir, "nested", "dirs", "path")
	err = CreateDirIfNotExist(nestedDir)
	if err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	// Check if nested directory exists
	info, err = os.Stat(nestedDir)
	if err != nil {
		t.Fatalf("Failed to stat nested directory: %v", err)
	}

	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", nestedDir)
	}

	// Test with empty path
	err = CreateDirIfNotExist("")
	if err != nil {
		// Empty path might be treated as current directory on some systems
		t.Logf("Creating directory with empty path: %v", err)
	}

	// Test creating directory where a file exists
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Trying to create directory with same name as file should fail
	err = CreateDirIfNotExist(filePath)
	if err == nil {
		t.Errorf("Expected error when creating directory with same name as file, got none")
	}

	// Test with inaccessible parent directory
	// Skip on Windows as permission handling is different
	if runtime.GOOS != "windows" {
		noAccessDir := filepath.Join(tmpDir, "noaccess")
		if err := os.Mkdir(noAccessDir, 0); err != nil {
			t.Logf("Could not create directory with no permissions: %v", err)
		} else {
			// Try to create directory inside the no-access directory
			inaccessibleDir := filepath.Join(noAccessDir, "child")
			err = CreateDirIfNotExist(inaccessibleDir)
			if err == nil {
				t.Errorf("Expected error when creating directory inside inaccessible parent, got none")
			}
			
			// Restore permissions at end
			os.Chmod(noAccessDir, 0755)
		}
	}
}

func TestPathExists(t *testing.T) {
	// Create temporary directory and file
	tmpDir, err := os.MkdirTemp("", "pathexists_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test existing file
	exists, err := PathExists(filePath)
	if err != nil {
		t.Fatalf("Unexpected error checking existing file: %v", err)
	}
	if !exists {
		t.Errorf("Expected existing file to return true")
	}

	// Test existing directory
	exists, err = PathExists(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error checking existing directory: %v", err)
	}
	if !exists {
		t.Errorf("Expected existing directory to return true")
	}

	// Test non-existent path
	nonExistentPath := filepath.Join(tmpDir, "nonexistent.txt")
	exists, err = PathExists(nonExistentPath)
	if err != nil {
		t.Fatalf("Unexpected error checking non-existent path: %v", err)
	}
	if exists {
		t.Errorf("Expected non-existent path to return false")
	}

	// Test empty path
	exists, err = PathExists("")
	if err != nil {
		t.Fatalf("Unexpected error checking empty path: %v", err)
	}
	// Empty path might be treated as current directory on some systems
	t.Logf("Empty path exists: %v", exists)

	// Test permission error by creating a directory with no access permissions
	// This might not work on Windows
	if runtime.GOOS != "windows" {
		noAccessDir := filepath.Join(tmpDir, "noaccess")
		if err := os.Mkdir(noAccessDir, 0000); err != nil {
			t.Logf("Could not create directory with no permissions: %v", err)
		} else {
			// Restore permissions at end of test
			defer os.Chmod(noAccessDir, 0755)
			
			noAccessFile := filepath.Join(noAccessDir, "file.txt")
			exists, err = PathExists(noAccessFile)
			// Might get an error or might just return false, both are ok
			if err != nil {
				t.Logf("Got expected permission error: %v", err)
			}
			if exists {
				t.Errorf("Expected inaccessible path to not exist or return error")
			}
		}
	}

	// Test with symbolic link if platform supports it
	symlinkPath := filepath.Join(tmpDir, "symlink.txt")
	os.Remove(symlinkPath) // Remove if exists
	err = os.Symlink(filePath, symlinkPath)
	if err != nil {
		t.Logf("Symlink creation not supported on this platform, skipping: %v", err)
	} else {
		exists, err = PathExists(symlinkPath)
		if err != nil {
			t.Errorf("Unexpected error checking symlink path: %v", err)
		}
		if !exists {
			t.Errorf("Expected symlink to exist")
		}

		// Test broken symlink
		brokenSymlinkPath := filepath.Join(tmpDir, "broken.txt")
		os.Remove(brokenSymlinkPath) // Remove if exists
		err = os.Symlink(filepath.Join(tmpDir, "nonexistent.txt"), brokenSymlinkPath)
		if err != nil {
			t.Logf("Broken symlink creation not supported, skipping: %v", err)
		} else {
			exists, err = PathExists(brokenSymlinkPath)
			// This is OS dependent, so just log the result
			t.Logf("Broken symlink exists: %v, err: %v", exists, err)
		}
	}
}

// Helper function to compare two byte slices
func equalByteSlices(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestHelperFunctions tests the helper functions in fileutils_test.go
func TestHelperFunctions(t *testing.T) {
	// Test equalByteSlices
	slice1 := []byte{1, 2, 3, 4}
	slice2 := []byte{1, 2, 3, 4}
	slice3 := []byte{1, 2, 3, 5}
	slice4 := []byte{1, 2, 3}
	
	if !equalByteSlices(slice1, slice2) {
		t.Errorf("Expected equal slices to return true")
	}
	
	if equalByteSlices(slice1, slice3) {
		t.Errorf("Expected different slices to return false")
	}
	
	if equalByteSlices(slice1, slice4) {
		t.Errorf("Expected slices of different lengths to return false")
	}
}
