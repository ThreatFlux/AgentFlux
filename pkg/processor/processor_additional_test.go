package processor

import (
	"fmt"
	"github.com/vtriple/agentflux/pkg/common/logging"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetLogger_Direct tests the SetLogger method directly
func TestSetLogger_Direct(t *testing.T) {
	// Create a processor
	processor := NewHashProcessor("sha256", 1)
	
	// Create a custom logger
	logger := logging.NewLogger("test-processor")
	
	// Set the logger
	processor.SetLogger(logger)
	
	// Verify the logger was set correctly
	if processor.logger != logger {
		t.Errorf("Expected logger to be set to custom logger")
	}
}

// TestScanStrings tests the custom scanner split function
func TestScanStrings(t *testing.T) {
	processor := NewHashProcessor("sha256", 1)
	
	tests := []struct {
		name           string
		input          string
		atEOF          bool
		expectedToken  string
		expectedAdvance int
		expectNil      bool
	}{
		{
			name:           "Simple string",
			input:          "HelloWorld",
			atEOF:          false,
			expectedToken:  "HelloWorld",
			expectedAdvance: 10,
			expectNil:      false,
		},
		{
			name:           "String with non-printable prefix",
			input:          "\000\001\002HelloWorld",
			atEOF:          false,
			expectedToken:  "HelloWorld",
			expectedAdvance: 13,
			expectNil:      false,
		},
		{
			name:           "String with non-printable suffix",
			input:          "HelloWorld\000\001\002",
			atEOF:          false,
			expectedToken:  "HelloWorld",
			expectedAdvance: 10,
			expectNil:      false,
		},
		{
			name:           "Empty string at EOF",
			input:          "",
			atEOF:          true,
			expectedToken:  "",
			expectedAdvance: 0,
			expectNil:      true,
		},
		{
			name:           "Only non-printable chars at EOF",
			input:          "\000\001\002",
			atEOF:          true,
			expectedToken:  "",
			expectedAdvance: 3,
			expectNil:      true,
		},
		{
			name:           "Non-printable chars not at EOF",
			input:          "\000\001\002",
			atEOF:          false,
			expectedToken:  "",
			expectedAdvance: 3,
			expectNil:      true,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			advance, token, err := processor.scanStrings([]byte(tc.input), tc.atEOF)
			
			// Check error
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			
			// Check advance
			if advance != tc.expectedAdvance {
				t.Errorf("Expected advance %d, got %d", tc.expectedAdvance, advance)
			}
			
			// Check token
			if tc.expectNil {
				if token != nil {
					t.Errorf("Expected nil token, got: %s", string(token))
				}
			} else {
				if token == nil {
					t.Errorf("Expected non-nil token")
				} else if string(token) != tc.expectedToken {
					t.Errorf("Expected token '%s', got '%s'", tc.expectedToken, string(token))
				}
			}
		})
	}
}

// TestExtractStringsError tests extraction with a reader that returns an error
func TestExtractStringsError(t *testing.T) {
	processor := NewHashProcessor("sha256", 1)
	
	// Create an error reader
	reader := &errorReader{err: io.ErrUnexpectedEOF}
	
	// Extract strings should fail
	_, err := processor.extractStrings(reader)
	if err == nil {
		t.Error("Expected error from extractStrings, got nil")
	}
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Expected io.ErrUnexpectedEOF, got: %v", err)
	}
}

// Helper type for tests
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

// TestProcessWithEmptyChannel tests the Process method with an empty channel
func TestProcessWithEmptyChannel(t *testing.T) {
	// Create processor
	processor := NewHashProcessor("sha256", 2)
	
	// Create empty channel and close it
	fileChannel := make(chan string)
	close(fileChannel)
	
	// Process files
	resultChannel := processor.Process(fileChannel)
	
	// Count results
	resultCount := 0
	for range resultChannel {
		resultCount++
	}
	
	// Verify no results
	if resultCount != 0 {
		t.Errorf("Expected 0 results for empty channel, got %d", resultCount)
	}
}

// TestProcessSingleWorker tests processing with a single worker
func TestProcessSingleWorker(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "single_worker_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create multiple test files
	filePaths := make([]string, 5)
	for i := 0; i < 5; i++ {
		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		content := []byte(fmt.Sprintf("Content for file %d", i))
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		filePaths[i] = filePath
	}
	
	// Create processor with single worker
	processor := NewHashProcessor("sha256", 1)
	
	// Create input channel
	fileChannel := make(chan string, len(filePaths))
	for _, path := range filePaths {
		fileChannel <- path
	}
	close(fileChannel)
	
	// Process files
	resultChannel := processor.Process(fileChannel)
	
	// Collect results
	results := make(map[string]FileResult)
	for result := range resultChannel {
		results[result.Path] = result
	}
	
	// Verify all files were processed
	if len(results) != len(filePaths) {
		t.Errorf("Expected %d results, got %d", len(filePaths), len(results))
	}
	
	// Verify each file was processed correctly
	for _, path := range filePaths {
		if result, ok := results[path]; ok {
			if result.Error != "" {
				t.Errorf("Expected no error for %s, got: %s", path, result.Error)
			}
			if result.Hash == "" {
				t.Errorf("Expected hash to be calculated for %s", path)
			}
		} else {
			t.Errorf("No result found for %s", path)
		}
	}
}

// TestProcessWithLargeFileSkipping tests the file size skipping functionality
func TestProcessWithLargeFileSkipping(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "file_size_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a test file
	smallFilePath := filepath.Join(tempDir, "small.txt")
	if err := os.WriteFile(smallFilePath, []byte("Small file content"), 0644); err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}
	
	// Create processor with very small max file size
	processor := NewHashProcessor("sha256", 1)
	processor.SkipLargeFiles = true
	processor.MaxFileSize = 10 // Just 10 bytes
	
	// Process the file
	result := processor.processFile(smallFilePath)
	
	// Verify the file was skipped due to size
	if result.Error == "" || !strings.Contains(result.Error, "file too large") {
		t.Errorf("Expected 'file too large' error, got: %s", result.Error)
	}
	
	// Test with skipping disabled
	processor.SkipLargeFiles = false
	result = processor.processFile(smallFilePath)
	
	// Verify the file was processed
	if result.Error != "" {
		t.Errorf("Expected no error with skipping disabled, got: %s", result.Error)
	}
	if result.Hash == "" {
		t.Error("Expected hash to be calculated with skipping disabled")
	}
}

// TestProcessFile_Errors tests various error conditions in processFile
func TestProcessFile_Errors(t *testing.T) {
	processor := NewHashProcessor("sha256", 1)
	
	// Test with non-existent file
	result := processor.processFile("/non/existent/file.txt")
	if result.Error == "" || !strings.Contains(result.Error, "stat error") {
		t.Errorf("Expected stat error for non-existent file, got: %s", result.Error)
	}
	
	// Test with inaccessible file
	if os.Geteuid() != 0 { // Skip if running as root
		tempFile, err := os.CreateTemp("", "no_permission")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tempPath := tempFile.Name()
		tempFile.Close()
		
		// Remove all permissions
		if err := os.Chmod(tempPath, 0); err != nil {
			t.Fatalf("Failed to change file permissions: %v", err)
		}
		
		// Try to process the file
		result = processor.processFile(tempPath)
		
		// Restore permissions for cleanup
		os.Chmod(tempPath, 0600)
		os.Remove(tempPath)
		
		// Verify the file open error
		if result.Error == "" || !strings.Contains(result.Error, "open error") {
			t.Errorf("Expected open error for inaccessible file, got: %s", result.Error)
		}
	}
}

// TestCalculateHashWithUnsupportedAlgorithm tests handling of unsupported hash algorithms
func TestCalculateHashWithUnsupportedAlgorithm(t *testing.T) {
	processor := NewHashProcessor("invalid-algorithm", 1)
	
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "hash_test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	
	// Write some data
	if _, err := tempFile.WriteString("Test content for hash"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()
	
	// Process the file
	result := processor.processFile(tempFile.Name())
	
	// Verify error
	if result.Error == "" || !strings.Contains(result.Error, "unsupported hash algorithm") {
		t.Errorf("Expected unsupported algorithm error, got: %s", result.Error)
	}
}

// TestCalculateHashErrors tests error cases for calculateHash
func TestCalculateHashErrors(t *testing.T) {
	processor := NewHashProcessor("sha256", 1)
	
	// Test with a reader that returns an error
	reader := &errorReader{err: io.ErrUnexpectedEOF}
	
	_, err := processor.calculateHash(reader)
	if err == nil {
		t.Error("Expected error from calculateHash with error reader, got nil")
	}
	
	// Test with an unsupported hash algorithm
	processor.HashAlgorithm = "invalid-algo"
	_, err = processor.calculateHash(strings.NewReader("test content"))
	if err == nil {
		t.Error("Expected error with invalid hash algorithm, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported hash algorithm") {
		t.Errorf("Expected 'unsupported hash algorithm' error, got: %v", err)
	}
}

// TestWorkerProcessing tests worker behavior with errors
func TestWorkerProcessing(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "worker_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("Test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create non-existent path
	nonExistentFile := filepath.Join(tempDir, "nonexistent.txt")
	
	// Create processor with high worker count to ensure all files processed
	processor := NewHashProcessor("sha256", 4)
	
	// Create input channel with mix of valid and invalid files
	fileChannel := make(chan string, 2)
	fileChannel <- testFile
	fileChannel <- nonExistentFile
	close(fileChannel)
	
	// Process files
	resultChannel := processor.Process(fileChannel)
	
	// Collect results
	results := make(map[string]FileResult)
	for result := range resultChannel {
		results[result.Path] = result
	}
	
	// Verify results count
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	
	// Verify results for valid file
	if result, ok := results[testFile]; ok {
		if result.Error != "" {
			t.Errorf("Expected no error for valid file, got: %s", result.Error)
		}
		if result.Hash == "" {
			t.Error("Expected non-empty hash for valid file")
		}
	} else {
		t.Errorf("No result found for valid file: %s", testFile)
	}
	
	// Verify results for non-existent file
	if result, ok := results[nonExistentFile]; ok {
		if result.Error == "" {
			t.Error("Expected error for non-existent file, got none")
		}
	} else {
		t.Errorf("No result found for non-existent file: %s", nonExistentFile)
	}
}

// TestExtractStringsWithLargeNumber tests extracting a large number of strings
func TestExtractStringsWithLargeNumber(t *testing.T) {
	// Create temp file
	tempDir, err := os.MkdirTemp("", "extract_strings_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create content with many unique strings
	var sb strings.Builder
	for i := 0; i < 15000; i++ {
		sb.WriteString(fmt.Sprintf("UniqueString%d\000", i))
	}
	
	testFile := filepath.Join(tempDir, "test.bin")
	if err := os.WriteFile(testFile, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create processor
	processor := NewHashProcessor("sha256", 1)
	processor.StringMinLength = 4
	processor.ExtractStrings = true
	
	// Open file
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()
	
	// Extract strings
	strings, err := processor.extractStrings(file)
	if err != nil {
		t.Fatalf("Failed to extract strings: %v", err)
	}
	
	// Check that we got the maximum number of strings
	maxStrings := 10000 // As defined in extractStrings
	if len(strings) != maxStrings {
		t.Errorf("Expected exactly %d strings due to limit, got %d", maxStrings, len(strings))
	}
}

// TestHashCalculationError tests when the hash calculation fails
func TestHashCalculationError(t *testing.T) {
	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "hash_error_test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	
	// Write some test data
	data := "Test content for hash calculation error"
	if _, err := tempFile.WriteString(data); err != nil {
		t.Fatalf("Failed to write data to temp file: %v", err)
	}
	tempFile.Close()
	
	// Create a processor with an invalid hash algorithm
	processor := NewHashProcessor("invalid-algorithm", 1)
	processor.ExtractStrings = true // Enable string extraction
	
	// Process the file - should fail during hash calculation
	result := processor.processFile(tempFile.Name())
	
	// Verify the error contains information about the hash algorithm
	if result.Error == "" || !strings.Contains(result.Error, "hash error") {
		t.Errorf("Expected hash error, got: %s", result.Error)
	}
	
	// Hash should be empty
	if result.Hash != "" {
		t.Errorf("Expected empty hash due to error, got: %s", result.Hash)
	}
	
	// Strings should be empty since extraction would be skipped after hash failure
	if len(result.Strings) > 0 {
		t.Errorf("Expected no strings due to hash calculation error, got %d strings", len(result.Strings))
	}
}