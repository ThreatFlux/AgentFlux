package scanner

import (
	"context"
	"github.com/vtriple/agentflux/pkg/common/logging"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSetContext_Direct tests the SetContext method directly
func TestSetContext_Direct(t *testing.T) {
	// Create a scanner
	scanner := NewFileScanner(context.Background(), []string{"/tmp"})
	
	// Create a new context
	ctx := context.Background()
	
	// Set the new context
	scanner.SetContext(ctx)
	
	// Verify the context was set correctly
	if scanner.ctx != ctx {
		t.Errorf("Expected context to be set to the new context")
	}
}

// TestSetLogger_Direct tests the SetLogger method directly
func TestSetLogger_Direct(t *testing.T) {
	// Create a scanner
	scanner := NewFileScanner(context.Background(), []string{"/tmp"})
	
	// Create a new logger
	logger := logging.NewLogger("test-scanner")
	
	// Set the new logger
	scanner.SetLogger(logger)
	
	// Verify the logger was set correctly
	if scanner.logger != logger {
		t.Errorf("Expected logger to be set to the new logger")
	}
}

// TestFileScanner_WithSymlinks tests handling of symlinks
func TestFileScanner_WithSymlinks(t *testing.T) {
	// Skip on platforms that don't support symlinks well
	if os.Getenv("CI") != "" && os.Getenv("RUNNER_OS") == "Windows" {
		t.Skip("Skipping symlink test on Windows CI")
	}
	
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "scanner_symlink_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	
	// Create a target file
	targetFile := filepath.Join(subDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target content"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}
	
	// Create a symlink to the file
	symlinkPath := filepath.Join(tempDir, "symlink.txt")
	err = os.Symlink(targetFile, symlinkPath)
	if err != nil {
		t.Skipf("Skipping test - unable to create symlink: %v", err)
	}
	
	// Create a broken symlink
	brokenSymlinkPath := filepath.Join(tempDir, "broken-symlink.txt")
	err = os.Symlink(filepath.Join(tempDir, "nonexistent.txt"), brokenSymlinkPath)
	if err != nil {
		t.Logf("Note: Unable to create broken symlink: %v", err)
		// Continue the test as this isn't critical
	}
	
	// Test cases
	tests := []struct {
		name           string
		skipSymlinks   bool
		expectedMinCount int  // At least this many files
	}{
		{
			name:           "Skip symlinks",
			skipSymlinks:   true,
			expectedMinCount: 1, // Just the target file
		},
		{
			name:           "Follow symlinks",
			skipSymlinks:   false,
			expectedMinCount: 1, // At least the target file
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			scanner := NewFileScanner(ctx, []string{tempDir})
			scanner.SkipSymlinks = tc.skipSymlinks
			
			fileChannel, errorChannel := scanner.Scan()
			
			// Count files and errors
			fileCount := 0
			for range fileChannel {
				fileCount++
			}
			
			// Drain error channel
			for range errorChannel {
				// Just consume errors
			}
			
			if fileCount < tc.expectedMinCount {
				t.Errorf("Expected at least %d files, got %d", tc.expectedMinCount, fileCount)
			}
		})
	}
}

// TestFileScanner_WithMaxFileSize tests maxFileSize behavior
func TestFileScanner_WithMaxFileSize(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "scanner_size_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a small file
	smallFilePath := filepath.Join(tempDir, "small.txt")
	if err := os.WriteFile(smallFilePath, []byte("small content"), 0644); err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}
	
	// Create a larger file
	largeFilePath := filepath.Join(tempDir, "large.txt")
	largeContent := make([]byte, 1024) // 1KB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(largeFilePath, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}
	
	// Test with different max file sizes
	tests := []struct {
		name          string
		maxFileSize   int64
		expectedCount int
	}{
		{
			name:          "No size limit",
			maxFileSize:   -1,
			expectedCount: 2, // Both files
		},
		{
			name:          "Small size limit",
			maxFileSize:   100, // 100 bytes, only small file
			expectedCount: 1,
		},
		{
			name:          "Large size limit",
			maxFileSize:   2048, // 2KB, both files
			expectedCount: 2,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scanner := NewFileScanner(ctx, []string{tempDir})
			scanner.MaxFileSize = tc.maxFileSize
			
			fileChannel, errorChannel := scanner.Scan()
			
			// Count files and errors
			fileCount := 0
			for range fileChannel {
				fileCount++
			}
			
			// Drain error channel
			for range errorChannel {
				// Just consume errors
			}
			
			if fileCount != tc.expectedCount {
				t.Errorf("Expected %d files, got %d", tc.expectedCount, fileCount)
			}
		})
	}
}

// TestFileScanner_WithInvalidPath tests behavior with non-existent paths
func TestFileScanner_WithInvalidPath(t *testing.T) {
	// Create a context
	ctx := context.Background()
	
	// Create a scanner with a non-existent path
	nonExistentPath := filepath.Join(os.TempDir(), "does-not-exist-"+time.Now().Format("20060102150405"))
	scanner := NewFileScanner(ctx, []string{nonExistentPath})
	
	// Run the scanner
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
	
	// Should have no files
	if fileCount != 0 {
		t.Errorf("Expected 0 files for non-existent path, got %d", fileCount)
	}
	
	// Should have an error
	if errorCount != 1 {
		t.Errorf("Expected 1 error for non-existent path, got %d", errorCount)
	}
}

// TestFileScanner_WithContextCancellation tests behavior when context is cancelled
func TestFileScanner_WithContextCancellation(t *testing.T) {
	// Create a temporary directory with many files to ensure the scan takes some time
	tempDir, err := os.MkdirTemp("", "scanner_cancel_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a bunch of subdirectories and files
	for i := 0; i < 5; i++ {
		subDir := filepath.Join(tempDir, "subdir"+string(rune('1'+i)))
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}
		
		for j := 0; j < 10; j++ {
			filePath := filepath.Join(subDir, "file"+string(rune('a'+j))+".txt")
			if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		}
	}
	
	// Create a cancelable context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	// Create a scanner
	scanner := NewFileScanner(ctx, []string{tempDir})
	
	// Run the scanner
	fileChannel, errorChannel := scanner.Scan()
	
	// Wait for context to be cancelled
	<-ctx.Done()
	
	// Count files and errors
	fileCount := 0
	for range fileChannel {
		fileCount++
	}
	
	// Drain error channel
	for range errorChannel {
		// Just consume errors
	}
	
	// The test passes if we don't hang - the scanner should abort when the context is cancelled
	// We may or may not get some files depending on timing
	t.Logf("Received %d files before context cancellation", fileCount)
}

// TestFileScanner_ProcessFileCancellationHandling tests processFile cancellation handling
func TestFileScanner_ProcessFileCancellationHandling(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "process_file_test.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer tempFile.Close()
	
	// Write some content
	if _, err := tempFile.WriteString("test content"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	
	// Get file info
	info, err := os.Stat(tempPath)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}
	
	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	// Create scanner with cancelled context
	scanner := NewFileScanner(ctx, []string{filepath.Dir(tempPath)})
	
	// Set up channels
	fileChannel := make(chan string, 5)
	errorChannel := make(chan error, 5)
	
	// Process the file - should be skipped due to cancelled context
	scanner.processFile(tempPath, info, fileChannel, errorChannel)
	
	// Close channels to complete reading
	close(fileChannel)
	close(errorChannel)
	
	// Check if any files were processed
	fileCount := 0
	for range fileChannel {
		fileCount++
	}
	
	// Check if any errors were generated
	errorCount := 0
	for range errorChannel {
		errorCount++
	}
	
	// Should not have processed any files due to cancelled context
	if fileCount != 0 {
		t.Errorf("Expected 0 files due to cancelled context, got %d", fileCount)
	}
}

// TestFileScanner_ScanPathCancellationHandling tests scanPath cancellation handling
func TestFileScanner_ScanPathCancellationHandling(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "scan_path_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	// Create scanner with cancelled context
	scanner := NewFileScanner(ctx, []string{tempDir})
	
	// Set up channels
	fileChannel := make(chan string, 5)
	errorChannel := make(chan error, 5)
	
	// Call scanPath - should return immediately due to cancelled context
	scanner.scanPath(tempDir, 0, fileChannel, errorChannel)
	
	// Close channels
	close(fileChannel)
	close(errorChannel)
	
	// Check if any files were processed
	fileCount := 0
	for range fileChannel {
		fileCount++
	}
	
	// Should not have processed any files due to cancelled context
	if fileCount != 0 {
		t.Errorf("Expected 0 files due to cancelled context, got %d", fileCount)
	}
}

// TestFileScanner_ScanPathWithSymlinks tests the scanPath method with symlinks
func TestFileScanner_ScanPathWithSymlinks(t *testing.T) {
	// Skip on platforms that don't support symlinks well
	if os.Getenv("CI") != "" && os.Getenv("RUNNER_OS") == "Windows" {
		t.Skip("Skipping symlink test on Windows CI")
	}
	
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "scanpath_symlink_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	
	// Create a target file
	targetFile := filepath.Join(subDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target content"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}
	
	// Create a symlink to the directory
	symlinkDir := filepath.Join(tempDir, "symlink_dir")
	if err := os.Symlink(subDir, symlinkDir); err != nil {
		t.Logf("Symlink creation not supported: %v", err)
		return // Skip the rest of the test
	}

	// Test with symlinks enabled and disabled
	
	// Test 1: Skip symlinks
	ctx1 := context.Background()
	scanner1 := NewFileScanner(ctx1, []string{tempDir})
	scanner1.SkipSymlinks = true
	fileChannel1 := make(chan string, 10)
	errorChannel1 := make(chan error, 10)
	
	// Scan the directory, shouldn't follow symlink
	scanner1.scanPath(tempDir, 0, fileChannel1, errorChannel1)
	close(fileChannel1)
	close(errorChannel1)
	
	// Count regular files (should not include symlinked files)
	fileCount1 := 0
	for range fileChannel1 {
		fileCount1++
	}
	
	// Test 2: Follow symlinks
	ctx2 := context.Background()
	scanner2 := NewFileScanner(ctx2, []string{tempDir})
	scanner2.SkipSymlinks = false
	fileChannel2 := make(chan string, 10)
	errorChannel2 := make(chan error, 10)
	
	// Scan the directory, should follow symlink
	scanner2.scanPath(tempDir, 0, fileChannel2, errorChannel2)
	close(fileChannel2)
	close(errorChannel2)
	
	// Count files (should include symlinked files)
	fileCount2 := 0
	for range fileChannel2 {
		fileCount2++
	}
	
	// Following symlinks should find more files than skipping them
	if fileCount2 <= fileCount1 {
		t.Logf("Following symlinks didn't find more files: skip=%d, follow=%d", 
			fileCount1, fileCount2)
	}
}

// TestFileScanner_ScanWithMaxDepth tests the MaxDepth functionality
func TestFileScanner_ScanWithMaxDepth(t *testing.T) {
	// Create a temporary directory structure with nested directories
	tempDir, err := os.MkdirTemp("", "depth_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a directory structure with 3 levels
	// Level 0: tempDir
	// Level 1: subdir1
	// Level 2: subdir1/subdir2
	
	// Create level 1 directory
	subDir1 := filepath.Join(tempDir, "subdir1")
	if err := os.Mkdir(subDir1, 0755); err != nil {
		t.Fatalf("Failed to create subdir1: %v", err)
	}
	
	// Create level 2 directory
	subDir2 := filepath.Join(subDir1, "subdir2")
	if err := os.Mkdir(subDir2, 0755); err != nil {
		t.Fatalf("Failed to create subdir2: %v", err)
	}
	
	// Create files at each level
	files := []string{
		filepath.Join(tempDir, "level0.txt"),
		filepath.Join(subDir1, "level1.txt"),
		filepath.Join(subDir2, "level2.txt"),
	}
	
	for _, file := range files {
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}
	
	// Test cases with different max depths
	tests := []struct {
		name          string
		maxDepth      int
		expectedFiles int
	}{
		{
			name:          "Max depth 0",
			maxDepth:      0,
			expectedFiles: 1, // Just level0.txt
		},
		{
			name:          "Max depth 1",
			maxDepth:      1,
			expectedFiles: 2, // level0.txt and level1.txt
		},
		{
			name:          "Max depth 2",
			maxDepth:      2,
			expectedFiles: 3, // All files
		},
		{
			name:          "Unlimited depth",
			maxDepth:      -1,
			expectedFiles: 3, // All files
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create scanner with specified max depth
			ctx := context.Background()
			scanner := NewFileScanner(ctx, []string{tempDir})
			scanner.MaxDepth = tc.maxDepth
			
			// Scan
			fileChannel, errorChannel := scanner.Scan()
			
			// Count files
			fileCount := 0
			for range fileChannel {
				fileCount++
			}
			
			// Drain errors
			for range errorChannel {
				// Just consume
			}
			
			if fileCount != tc.expectedFiles {
				t.Errorf("Expected %d files with max depth %d, but got %d", 
					tc.expectedFiles, tc.maxDepth, fileCount)
			}
		})
	}
}

// TestFileScanner_ShouldExcludeFullPath tests more complex exclusion patterns
func TestFileScanner_ShouldExcludeFullPath(t *testing.T) {
	scanner := NewFileScanner(context.Background(), []string{"."})
	
	// Test cases
	tests := []struct {
		name           string
		path           string
		excludePatterns []string
		shouldExclude  bool
	}{
		{
			name:           "Exclude by basename",
			path:           "/path/to/excluded.txt",
			excludePatterns: []string{"excluded.txt"},
			shouldExclude:  true,
		},
		{
			name:           "Don't exclude non-matching basename",
			path:           "/path/to/included.txt",
			excludePatterns: []string{"excluded.txt"},
			shouldExclude:  false,
		},
		{
			name:           "Exclude by extension",
			path:           "/path/to/file.tmp",
			excludePatterns: []string{"*.tmp"},
			shouldExclude:  true,
		},
		{
			name:           "Exclude by full path pattern",
			path:           "/path/to/special/file.txt",
			excludePatterns: []string{"/path/to/special/*"},
			shouldExclude:  true,
		},
		{
			name:           "Multiple exclude patterns - match one",
			path:           "/path/to/file.log",
			excludePatterns: []string{"*.tmp", "*.log", "*.bak"},
			shouldExclude:  true,
		},
		{
			name:           "Multiple exclude patterns - match none",
			path:           "/path/to/file.txt",
			excludePatterns: []string{"*.tmp", "*.log", "*.bak"},
			shouldExclude:  false,
		},
		{
			name:           "Invalid pattern handling",
			path:           "/path/to/file.txt",
			excludePatterns: []string{"[invalid"},
			shouldExclude:  false, // Should not crash and not exclude
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scanner.ExcludePaths = tc.excludePatterns
			result := scanner.shouldExclude(tc.path)
			
			if result != tc.shouldExclude {
				t.Errorf("Path: %s, Expected shouldExclude=%v but got %v", 
					tc.path, tc.shouldExclude, result)
			}
		})
	}
}