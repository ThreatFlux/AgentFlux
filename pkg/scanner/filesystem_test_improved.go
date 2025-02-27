package scanner

import (
	"context"
	"fmt"
	"github.com/vtriple/agentflux/pkg/common/logging"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestFileScanner_ScanExtended provides more extensive testing of the FileScanner
func TestFileScanner_ScanExtended(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "scanner_extended_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a more complex directory structure
	createTestDirectoryStructure(t, tempDir)

	// Test cases
	tests := []struct {
		name           string
		rootPaths      []string
		excludePaths   []string
		maxDepth       int
		maxFileSize    int64
		skipHidden     bool
		skipSymlinks   bool
		expectedFiles  int
		expectedErrors int
	}{
		{
			name:          "Multiple root paths",
			rootPaths:     []string{
				filepath.Join(tempDir, "dir1"),
				filepath.Join(tempDir, "dir2"),
			},
			maxDepth:      -1,
			skipHidden:    true,
			skipSymlinks:  true,
			expectedFiles: 4, // 2 in dir1 + 2 in dir2
		},
		{
			name:          "Exclude by pattern",
			rootPaths:     []string{tempDir},
			excludePaths:  []string{"*.log", "**/temp*"},
			maxDepth:      -1,
			skipHidden:    false,
			skipSymlinks:  true,
			// Expecting all files except *.log and files in temp directories
			expectedFiles: 8,
		},
		{
			name:          "Max file size",
			rootPaths:     []string{tempDir},
			maxDepth:      -1,
			maxFileSize:   10, // Only include files <= 10 bytes
			skipHidden:    false,
			skipSymlinks:  true,
			// Expecting only the smallest files
			expectedFiles: 3,
		},
		{
			name:          "Depth 1 only",
			rootPaths:     []string{tempDir},
			maxDepth:      1,
			skipHidden:    false,
			skipSymlinks:  true,
			// Only files directly in the root dirs (dir1, dir2, dir3)
			expectedFiles: 3,
		},
		{
			name:          "Include symlinks",
			rootPaths:     []string{tempDir},
			maxDepth:      -1,
			skipHidden:    true,
			skipSymlinks:  false,
			// Include symlinked files if platform supports them
			expectedFiles: getExpectedFileCount(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test context
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Create scanner
			scanner := NewFileScanner(ctx, tc.rootPaths)
			scanner.ExcludePaths = tc.excludePaths
			scanner.MaxDepth = tc.maxDepth
			scanner.MaxFileSize = tc.maxFileSize
			scanner.SkipHiddenFiles = tc.skipHidden
			scanner.SkipSymlinks = tc.skipSymlinks

			// Run scan
			fileChannel, errorChannel := scanner.Scan()

			// Count files and errors
			fileCount := 0
			var files []string
			for path := range fileChannel {
				fileCount++
				files = append(files, path)
			}

			errorCount := 0
			var errs []error
			for err := range errorChannel {
				errorCount++
				errs = append(errs, err)
			}

			// Log found files for debugging
			t.Logf("Found %d files: %v", fileCount, files)
			if len(errs) > 0 {
				t.Logf("Encountered %d errors: %v", errorCount, errs)
			}

			// Skip exact file count check for symlink tests on Windows
			if tc.skipSymlinks == false && runtime.GOOS == "windows" {
				t.Skip("Skipping exact file count check for symlink test on Windows")
			}

			// Check results
			if tc.expectedFiles > 0 && fileCount != tc.expectedFiles {
				t.Errorf("Expected %d files, got %d", tc.expectedFiles, fileCount)
			}

			if errorCount != tc.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tc.expectedErrors, errorCount)
			}
		})
	}
}

// Helper function to get expected file count based on platform
func getExpectedFileCount() int {
	if runtime.GOOS == "windows" {
		return 12
	}
	return 13
}

// TestFileScanner_ScanWithContextCancellation tests scanner behavior when context is cancelled
func TestFileScanner_ScanWithContextCancellation(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "scanner_context_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a large directory structure to ensure scanning takes some time
	createLargeTestDirectoryStructure(t, tempDir)

	// Create a context that will be cancelled soon
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Create scanner
	scanner := NewFileScanner(ctx, []string{tempDir})
	scanner.MaxDepth = -1

	// Run scan
	fileChannel, errorChannel := scanner.Scan()

	// Collect results until channels are closed
	fileCount := 0
	for range fileChannel {
		fileCount++
	}

	errorCount := 0
	for range errorChannel {
		errorCount++
	}

	// We expect the scan to be interrupted, so we should get fewer files than
	// the total number in the directory structure
	t.Logf("Received %d files before context cancellation", fileCount)

	// The exact number is non-deterministic, but we can check that the operation was interrupted
	totalFiles := countFilesInDirectory(t, tempDir)
	if fileCount >= totalFiles {
		t.Errorf("Expected context cancellation to interrupt scan (total files: %d, received: %d)",
			totalFiles, fileCount)
	}
}

// TestFileScanner_SetContext tests the SetContext method
func TestFileScanner_SetContext(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "scanner_setcontext_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a few test files
	for i := 0; i < 5; i++ {
		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create initial context and scanner
	ctx1, cancel1 := context.WithCancel(context.Background())
	scanner := NewFileScanner(ctx1, []string{tempDir})
	
	// Cancel the first context
	cancel1()
	
	// Create a new context
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	
	// Update the scanner with the new context
	scanner.SetContext(ctx2)
	
	// Now scanning should work with the new context
	fileChannel, errorChannel := scanner.Scan()
	
	// Count files
	fileCount := 0
	for range fileChannel {
		fileCount++
	}
	
	// Check errors
	for err := range errorChannel {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// Verify we got the expected number of files
	if fileCount != 5 {
		t.Errorf("Expected 5 files, got %d", fileCount)
	}
}

// TestFileScanner_SetLogger tests the SetLogger method
func TestFileScanner_SetLogger(t *testing.T) {
	// Create a test logger
	logger := logging.NewLogger("test_scanner")
	
	// Create scanner
	scanner := NewFileScanner(context.Background(), []string{"/tmp"})
	
	// Set logger
	scanner.SetLogger(logger)
	
	// Verify the logger was set (we can only check this indirectly)
	// This is mostly for coverage since we can't easily check the logger's value
	if scanner.logger == nil {
		t.Errorf("Expected logger to be set, got nil")
	}
}

// TestIsHiddenFile tests the isHiddenFile function
func TestIsHiddenFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{
			name:     "Regular file",
			filename: "regular.txt",
			expected: false,
		},
		{
			name:     "Hidden file",
			filename: ".hidden",
			expected: true,
		},
		{
			name:     "Current directory",
			filename: ".",
			expected: false,
		},
		{
			name:     "Parent directory",
			filename: "..",
			expected: false,
		},
		{
			name:     "Hidden file with extension",
			filename: ".gitignore",
			expected: true,
		},
		{
			name:     "File with dot in name",
			filename: "file.with.dots.txt",
			expected: false,
		},
		{
			name:     "Empty string",
			filename: "",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isHiddenFile(tc.filename)
			if result != tc.expected {
				t.Errorf("isHiddenFile(%s) = %v, want %v", tc.filename, result, tc.expected)
			}
		})
	}
}

// TestFileScanner_ShouldExclude_Advanced tests more complex exclude patterns
func TestFileScanner_ShouldExclude_Advanced(t *testing.T) {
	scanner := NewFileScanner(context.Background(), []string{"/test"})

	// Test cases with more complex patterns
	tests := []struct {
		name           string
		path           string
		excludePatterns []string
		expected       bool
	}{
		{
			name:           "Multiple patterns, no match",
			path:           "/test/file.txt",
			excludePatterns: []string{"*.log", "*.tmp", "backup/*"},
			expected:       false,
		},
		{
			name:           "Multiple patterns, one matches",
			path:           "/test/file.log",
			excludePatterns: []string{"*.log", "*.tmp", "backup/*"},
			expected:       true,
		},
		{
			name:           "Directory pattern",
			path:           "/test/backup/file.txt",
			excludePatterns: []string{"backup/*"},
			expected:       true,
		},
		{
			name:           "Exclude specific file",
			path:           "/test/specific.txt",
			excludePatterns: []string{"specific.txt"},
			expected:       true,
		},
		{
			name:           "Case sensitivity",
			path:           "/test/File.TXT",
			excludePatterns: []string{"*.txt"},
			expected:       false, // filepath.Match is case-sensitive
		},
		{
			name:           "Special characters in path",
			path:           "/test/file[special].txt",
			excludePatterns: []string{"file[special].txt"},
			expected:       false, // [ is a special character in glob patterns
		},
		{
			name:           "Escaped special characters in pattern",
			path:           "/test/file[special].txt",
			excludePatterns: []string{"file\\[special\\].txt"},
			expected:       true, // Escaped brackets
		},
		{
			name:           "Full path match attempt",
			path:           "/test/subdir/deep/file.txt",
			excludePatterns: []string{"/test/subdir/deep/file.txt"},
			expected:       true, // Full path can match
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scanner.ExcludePaths = tc.excludePatterns
			result := scanner.shouldExclude(tc.path)
			if result != tc.expected {
				t.Errorf("Expected shouldExclude to return %v for path %s with patterns %v, got %v",
					tc.expected, tc.path, tc.excludePatterns, result)
			}
		})
	}
}

// TestFileScanner_ProcessFile tests the processFile method
func TestFileScanner_ProcessFile(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "scanner_processfile_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files with different sizes
	smallFile := filepath.Join(tempDir, "small.txt")
	if err := os.WriteFile(smallFile, []byte("small"), 0644); err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}

	largeFile := filepath.Join(tempDir, "large.txt")
	largeContent := strings.Repeat("large content ", 1000) // ~12KB
	if err := os.WriteFile(largeFile, []byte(largeContent), 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	// Create a non-regular file (directory)
	dirPath := filepath.Join(tempDir, "directory")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Test cases
	tests := []struct {
		name          string
		path          string
		maxFileSize   int64
		shouldExclude bool
		expectSend    bool
	}{
		{
			name:          "Small file, no size limit",
			path:          smallFile,
			maxFileSize:   -1,
			shouldExclude: false,
			expectSend:    true,
		},
		{
			name:          "Small file, with sufficient size limit",
			path:          smallFile,
			maxFileSize:   100,
			shouldExclude: false,
			expectSend:    true,
		},
		{
			name:          "Large file, with small size limit",
			path:          largeFile,
			maxFileSize:   100,
			shouldExclude: false,
			expectSend:    false, // Too large for limit
		},
		{
			name:          "Directory, should be skipped",
			path:          dirPath,
			maxFileSize:   -1,
			shouldExclude: false,
			expectSend:    false, // Not a regular file
		},
		{
			name:          "Excluded file",
			path:          smallFile,
			maxFileSize:   -1,
			shouldExclude: true,
			expectSend:    false, // Excluded
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create context and channels
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			fileChannel := make(chan string, 1)
			errorChannel := make(chan error, 1)

			// Create scanner
			scanner := NewFileScanner(ctx, []string{tempDir})
			scanner.MaxFileSize = tc.maxFileSize

			// Mock shouldExclude method
			if tc.shouldExclude {
				// Add the file name to exclude paths
				scanner.ExcludePaths = []string{filepath.Base(tc.path)}
			} else {
				scanner.ExcludePaths = []string{}
			}

			// Get file info
			info, err := os.Stat(tc.path)
			if err != nil {
				t.Fatalf("Failed to stat test file: %v", err)
			}

			// Call processFile
			scanner.processFile(tc.path, info, fileChannel, errorChannel)

			// Check if file was sent to channel
			select {
			case receivedPath := <-fileChannel:
				if !tc.expectSend {
					t.Errorf("Expected file not to be sent, but received: %s", receivedPath)
				} else if receivedPath != tc.path {
					t.Errorf("Expected path %s, got %s", tc.path, receivedPath)
				}
			default:
				if tc.expectSend {
					t.Errorf("Expected file to be sent, but none received")
				}
			}

			// Check for errors
			select {
			case err := <-errorChannel:
				t.Errorf("Unexpected error: %v", err)
			default:
				// No error expected
			}
		})
	}
}

// Helper functions

// createTestDirectoryStructure creates a directory structure for testing the scanner
func createTestDirectoryStructure(t *testing.T, rootDir string) {
	// Create structure:
	// rootDir/
	//   ├── dir1/
	//   │   ├── file1.txt
	//   │   └── file2.log
	//   ├── dir2/
	//   │   ├── file3.txt
	//   │   └── subdir/
	//   │       └── file4.txt
	//   ├── dir3/
	//   │   └── .hidden_file.txt
	//   ├── tempdir/
	//   │   └── temp.txt
	//   └── .hidden_dir/
	//       └── hidden_file.txt

	// Create directories
	dirs := []string{
		filepath.Join(rootDir, "dir1"),
		filepath.Join(rootDir, "dir2"),
		filepath.Join(rootDir, "dir2", "subdir"),
		filepath.Join(rootDir, "dir3"),
		filepath.Join(rootDir, "tempdir"),
		filepath.Join(rootDir, ".hidden_dir"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create files with different sizes
	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(rootDir, "dir1", "file1.txt"), "This is file1"},
		{filepath.Join(rootDir, "dir1", "file2.log"), "This is a log file"},
		{filepath.Join(rootDir, "dir2", "file3.txt"), "This is file3 with more content"},
		{filepath.Join(rootDir, "dir2", "subdir", "file4.txt"), "Nested file4"},
		{filepath.Join(rootDir, "dir3", ".hidden_file.txt"), "This is a hidden file"},
		{filepath.Join(rootDir, "tempdir", "temp.txt"), "Temporary file"},
		{filepath.Join(rootDir, ".hidden_dir", "hidden_file.txt"), "File in hidden directory"},
	}

	for _, file := range files {
		if err := os.WriteFile(file.path, []byte(file.content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file.path, err)
		}
	}

	// Create a symlink if supported
	if runtime.GOOS != "windows" {
		srcPath := filepath.Join(rootDir, "dir1", "file1.txt")
		linkPath := filepath.Join(rootDir, "link_to_file1.txt")
		if err := os.Symlink(srcPath, linkPath); err != nil {
			t.Logf("Symlink creation not supported or failed: %v", err)
		}
	}
}

// createLargeTestDirectoryStructure creates a large directory structure for testing
func createLargeTestDirectoryStructure(t *testing.T, rootDir string) {
	// Create a structure with many nested directories and files
	for i := 0; i < 5; i++ {
		dirPath := filepath.Join(rootDir, fmt.Sprintf("dir%d", i))
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dirPath, err)
		}

		for j := 0; j < 10; j++ {
			subDirPath := filepath.Join(dirPath, fmt.Sprintf("subdir%d", j))
			if err := os.MkdirAll(subDirPath, 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", subDirPath, err)
			}

			for k := 0; k < 5; k++ {
				filePath := filepath.Join(subDirPath, fmt.Sprintf("file%d.txt", k))
				content := fmt.Sprintf("Content for file %d in subdir %d of dir %d", k, j, i)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create file %s: %v", filePath, err)
				}
			}
		}
	}
}

// countFilesInDirectory recursively counts files in a directory
func countFilesInDirectory(t *testing.T, dir string) int {
	count := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to count files: %v", err)
	}
	return count
}
