package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileScanner_Scan(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "scanner_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files and directories
	files := []string{
		"file1.txt",
		"file2.bin",
		"subdir/nested.txt",
		".hiddenfile",
	}

	// Create the files
	for _, file := range files {
		path := filepath.Join(tempDir, file)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Create test context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test cases
	tests := []struct {
		name           string
		rootPaths      []string
		excludePaths   []string
		maxDepth       int
		skipHidden     bool
		skipSymlinks   bool
		expectedCount  int
		expectedErrors int
	}{
		{
			name:          "Scan all files",
			rootPaths:     []string{tempDir},
			maxDepth:      -1,
			skipHidden:    false,
			skipSymlinks:  true,
			expectedCount: 4, // All 4 files
		},
		{
			name:          "Skip hidden files",
			rootPaths:     []string{tempDir},
			maxDepth:      -1,
			skipHidden:    true,
			skipSymlinks:  true,
			expectedCount: 3, // All files except .hiddenfile
		},
		{
			name:          "Limit depth to 0",
			rootPaths:     []string{tempDir},
			maxDepth:      0,
			skipHidden:    false,
			skipSymlinks:  true,
			expectedCount: 3, // Only files in root dir, including .hiddenfile
		},
		{
			name:          "Exclude *.bin files",
			rootPaths:     []string{tempDir},
			excludePaths:  []string{"*.bin"},
			maxDepth:      -1,
			skipHidden:    false,
			skipSymlinks:  true,
			expectedCount: 3, // All files except file2.bin
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create scanner
			scanner := NewFileScanner(ctx, tc.rootPaths)
			scanner.ExcludePaths = tc.excludePaths
			scanner.MaxDepth = tc.maxDepth
			scanner.SkipHiddenFiles = tc.skipHidden
			scanner.SkipSymlinks = tc.skipSymlinks

			// Run scan
			fileChannel, errorChannel := scanner.Scan()

			// Count files and errors
			fileCount := 0
			for range fileChannel {
				fileCount++
			}

			errorCount := 0
			for range errorChannel {
				errorCount++
			}

			// Check results
			if fileCount != tc.expectedCount {
				t.Errorf("Expected %d files, got %d", tc.expectedCount, fileCount)
			}

			if errorCount != tc.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tc.expectedErrors, errorCount)
			}
		})
	}
}

func TestFileScanner_ShouldExclude(t *testing.T) {
	scanner := NewFileScanner(context.Background(), []string{"/test"})

	// Test cases
	tests := []struct {
		name          string
		path          string
		excludePatterns []string
		expected      bool
	}{
		{
			name:          "No exclude patterns",
			path:          "/test/file.txt",
			excludePatterns: []string{},
			expected:      false,
		},
		{
			name:          "Exclude by extension",
			path:          "/test/file.txt",
			excludePatterns: []string{"*.txt"},
			expected:      true,
		},
		{
			name:          "Exclude by name",
			path:          "/test/temp.log",
			excludePatterns: []string{"temp.*"},
			expected:      true,
		},
		{
			name:          "Non-matching pattern",
			path:          "/test/file.txt",
			excludePatterns: []string{"*.log", "*.tmp"},
			expected:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scanner.ExcludePaths = tc.excludePatterns
			result := scanner.shouldExclude(tc.path)
			if result != tc.expected {
				t.Errorf("Expected shouldExclude to return %v, got %v", tc.expected, result)
			}
		})
	}
}
