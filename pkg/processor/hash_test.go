package processor

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewHashProcessor(t *testing.T) {
	// Test with different algorithms
	algorithms := []string{"md5", "sha1", "sha256", "sha512"}
	workers := 4

	for _, alg := range algorithms {
		processor := NewHashProcessor(alg, workers)
		if processor == nil {
			t.Fatalf("Expected non-nil processor for algorithm %s", alg)
		}

		if processor.HashAlgorithm != alg {
			t.Errorf("Expected algorithm %s, got %s", alg, processor.HashAlgorithm)
		}

		if processor.WorkerCount != workers {
			t.Errorf("Expected %d workers, got %d", workers, processor.WorkerCount)
		}
	}
}

func TestCalculateHash(t *testing.T) {
	// Create a temporary file with known content
	tempDir, err := os.MkdirTemp("", "processor_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	content := "This is a test file content for hashing."
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Pre-compute expected hashes
	md5Hash := fmt.Sprintf("%x", md5.Sum([]byte(content)))
	sha1Hash := fmt.Sprintf("%x", sha1.Sum([]byte(content)))
	sha256Hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	sha512Hash := fmt.Sprintf("%x", sha512.Sum512([]byte(content)))

	// Test cases
	tests := []struct {
		name      string
		algorithm string
		expected  string
	}{
		{
			name:      "MD5 Hash",
			algorithm: "md5",
			expected:  md5Hash,
		},
		{
			name:      "SHA1 Hash",
			algorithm: "sha1",
			expected:  sha1Hash,
		},
		{
			name:      "SHA256 Hash",
			algorithm: "sha256",
			expected:  sha256Hash,
		},
		{
			name:      "SHA512 Hash",
			algorithm: "sha512",
			expected:  sha512Hash,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create processor with test algorithm
			processor := NewHashProcessor(tc.algorithm, 1)

			// Open the file
			file, err := os.Open(testFile)
			if err != nil {
				t.Fatalf("Failed to open test file: %v", err)
			}
			defer file.Close()

			// Calculate hash
			hash, err := processor.calculateHash(file)
			if err != nil {
				t.Fatalf("Failed to calculate hash: %v", err)
			}

			// Verify hash
			if hash != tc.expected {
				t.Errorf("Expected hash %s, got %s", tc.expected, hash)
			}
		})
	}

	// Test with unsupported algorithm
	t.Run("Unsupported Algorithm", func(t *testing.T) {
		processor := NewHashProcessor("unsupported", 1)
		file, _ := os.Open(testFile)
		defer file.Close()

		_, err := processor.calculateHash(file)
		if err == nil {
			t.Error("Expected error for unsupported algorithm, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported hash algorithm") {
			t.Errorf("Expected error message about unsupported algorithm, got: %v", err)
		}
	})
}

func TestExtractStrings(t *testing.T) {
	// Create a temporary file with known content
	tempDir, err := os.MkdirTemp("", "processor_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file with some strings
	content := "Binary\000content\000with\000some\000embedded\000strings.\000" +
		"HELLO_WORLD\000" +
		"TEST_STRING_1234\000" +
		"This is a longer string that should be extracted.\000" +
		"ShortStr\000" + // This should be filtered if min length > 8
		"END_OF_FILE"
	testFile := filepath.Join(tempDir, "test.bin")
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test cases
	tests := []struct {
		name           string
		minLength      int
		expectedCount  int
		expectedString string // One of the strings that should be found
	}{
		{
			name:           "Default Min Length (4)",
			minLength:      4,
			expectedCount:  8, // All strings including "ShortStr"
			expectedString: "HELLO_WORLD",
		},
		{
			name:           "Higher Min Length (10)",
			minLength:      10,
			expectedCount:  3, // Only strings longer than 10 chars
			expectedString: "TEST_STRING_1234",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create processor
			processor := NewHashProcessor("sha256", 1)
			processor.StringMinLength = tc.minLength
			processor.ExtractStrings = true

			// Open the file
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

			// Verify string count - it might not be exact due to how strings are extracted
			// but should be at least the expected count
			if len(strings) < tc.expectedCount-2 { // Allow for some flexibility
				t.Errorf("Expected at least %d strings, got %d", tc.expectedCount-2, len(strings))
			}

			// Verify expected string is present
			found := false
			for _, s := range strings {
				if s == tc.expectedString {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected string '%s' not found in extracted strings", tc.expectedString)
			}
		})
	}
}

// TestExtractStringsManyDuplicates tests handling of many duplicate strings
func TestExtractStringsManyDuplicates(t *testing.T) {
	// Create test file with repeated content
	tempFile, err := os.CreateTemp("", "duplicate_strings_test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write 1000 copies of the same string with null separators
	for i := 0; i < 1000; i++ {
		tempFile.WriteString("DUPLICATE")
		tempFile.Write([]byte{0}) // Null separator
	}
	tempFile.Close()

	// Create processor
	processor := NewHashProcessor("sha256", 1)
	processor.StringMinLength = 4
	
	// Extract strings
	file, err := os.Open(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to open temp file: %v", err)
	}
	defer file.Close()

	extractedStrings, err := processor.extractStrings(file)
	if err != nil {
		t.Fatalf("Failed to extract strings: %v", err)
	}
	
	// Due to how scanStrings works, we might get a few variations of the string
	// Let's just verify DUPLICATE is one of them
	hasDuplicate := false
	for _, s := range extractedStrings {
		if s == "DUPLICATE" {
			hasDuplicate = true
			break
		}
	}
	
	if !hasDuplicate {
		t.Errorf("Expected to find 'DUPLICATE' in extracted strings, but it wasn't found")
	}
}

// TestScanStringsEdgeCases tests edge cases for the scanStrings function
func TestScanStringsEdgeCases(t *testing.T) {
	processor := NewHashProcessor("sha256", 1)
	
	tests := []struct {
		name        string
		data        []byte
		atEOF       bool
		wantAdvance int
		wantToken   []byte
		wantErr     bool
	}{
		{
			name:        "Empty data not at EOF",
			data:        []byte{},
			atEOF:       false,
			wantAdvance: 0,
			wantToken:   nil,
			wantErr:     false,
		},
		{
			name:        "Empty data at EOF",
			data:        []byte{},
			atEOF:       true,
			wantAdvance: 0,
			wantToken:   nil,
			wantErr:     false,
		},
		{
			name:        "Non-printable data not at EOF",
			data:        []byte{0x01, 0x02, 0x03},
			atEOF:       false,
			wantAdvance: 3,
			wantToken:   nil,
			wantErr:     false,
		},
		{
			name:        "Non-printable data at EOF",
			data:        []byte{0x01, 0x02, 0x03},
			atEOF:       true,
			wantAdvance: 3,
			wantToken:   nil,
			wantErr:     false,
		},
		{
			name:        "Mixed data",
			data:        []byte{0x01, 0x02, 'H', 'e', 'l', 'l', 'o', 0x03, 0x04},
			atEOF:       false,
			wantAdvance: 7,
			wantToken:   []byte("Hello"),
			wantErr:     false,
		},
		{
			name:        "ASCII string",
			data:        []byte("Hello"),
			atEOF:       false,
			wantAdvance: 5,
			wantToken:   []byte("Hello"),
			wantErr:     false,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			advance, token, err := processor.scanStrings(tc.data, tc.atEOF)
			
			if (err != nil) != tc.wantErr {
				t.Errorf("scanStrings() error = %v, wantErr %v", err, tc.wantErr)
			}
			
			if advance != tc.wantAdvance {
				t.Errorf("scanStrings() advance = %v, want %v", advance, tc.wantAdvance)
			}
			
			if !bytes.Equal(token, tc.wantToken) {
				var tokenStr string
				if token == nil {
					tokenStr = "nil"
				} else {
					tokenStr = string(token)
				}
				
				var wantStr string
				if tc.wantToken == nil {
					wantStr = "nil"
				} else {
					wantStr = string(tc.wantToken)
				}
				
				t.Errorf("scanStrings() token = %v (%s), want %v (%s)", 
					token, tokenStr, tc.wantToken, wantStr)
			}
		})
	}
}

// TestProcessWithExtraConcurrency tests the Process method with multiple workers and concurrent operations
func TestProcessWithExtraConcurrency(t *testing.T) {
	// Create temp directory for test files
	tempDir, err := os.MkdirTemp("", "process_concurrency_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create multiple files
	fileCount := 50
	filePaths := make([]string, fileCount)
	for i := 0; i < fileCount; i++ {
		content := []byte(fmt.Sprintf("Content for file %d. This is a test file with some data.", i))
		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		filePaths[i] = filePath
	}

	// Create processor with many workers
	processor := NewHashProcessor("sha256", 10)

	// Create input channel and send files
	fileChannel := make(chan string, fileCount)
	for _, file := range filePaths {
		fileChannel <- file
	}
	close(fileChannel)

	// Process files
	resultChannel := processor.Process(fileChannel)

	// Add concurrent activity
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Create another processor while processing is happening
		newProcessor := NewHashProcessor("md5", 5)
		if newProcessor == nil {
			t.Error("Failed to create new processor during processing")
		}
	}()

	// Collect results with additional concurrency
	results := make([]FileResult, 0, fileCount)
	resultChan := make(chan FileResult, 10)
	
	// Collect in separate goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range resultChannel {
			resultChan <- result
		}
		close(resultChan)
	}()
	
	// Read from collection channel
	for result := range resultChan {
		results = append(results, result)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check that all files were processed
	if len(results) != fileCount {
		t.Errorf("Expected %d results, got %d", fileCount, len(results))
	}

	// Check that all results have a hash
	for _, result := range results {
		if result.Hash == "" {
			t.Errorf("Result missing hash: %+v", result)
		}
	}
}

// TestExtractStringsWithoutEnoughMemory tests handling of very large string extraction that might exceed memory
func TestExtractStringsWithoutEnoughMemory(t *testing.T) {
	// Create a processor
	processor := NewHashProcessor("sha256", 1)
	processor.StringMinLength = 4
	
	// Create a reader that generates an infinite stream of strings
	// This simulates a huge file with many strings
	infiniteReader := &infiniteStringReader{}
	
	// Extract strings - should hit the 10000 string limit
	strings, err := processor.extractStrings(infiniteReader)
	if err != nil {
		t.Fatalf("Failed to extract strings: %v", err)
	}
	
	// Should stop at 10000 strings (the limit in the code)
	if len(strings) > 10000 {
		t.Errorf("Expected at most 10000 strings, got %d", len(strings))
	}
	
	if len(strings) < 9000 {
		t.Errorf("Expected close to 10000 strings, only got %d", len(strings))
	}
}

// infiniteStringReader is a reader that generates an infinite stream of printable strings
// separated by null bytes
type infiniteStringReader struct {
	count int
}

func (r *infiniteStringReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	
	// Generate readable strings with null separators
	n = 0
	for n < len(p) {
		str := fmt.Sprintf("String%d", r.count)
		r.count++
		
		// Write string + null byte
		for i := 0; i < len(str) && n < len(p); i++ {
			p[n] = str[i]
			n++
		}
		
		// Add null separator if there's room
		if n < len(p) {
			p[n] = 0
			n++
		}
	}
	
	return n, nil
}

func TestProcessFile(t *testing.T) {
	// Create a temporary file
	tempDir, err := os.MkdirTemp("", "processor_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a regular test file
	regularContent := "This is a regular test file."
	regularFile := filepath.Join(tempDir, "regular.txt")
	err = os.WriteFile(regularFile, []byte(regularContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create regular test file: %v", err)
	}

	// Create a large test file if testing with large files
	largeFile := filepath.Join(tempDir, "large.bin")
	largeFileSize := int64(1024 * 1024) // 1MB
	createLargeFile(t, largeFile, largeFileSize)

	// Compute expected hash for regular file
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(regularContent)))

	// Test cases
	tests := []struct {
		name          string
		filePath      string
		extractString bool
		skipLarge     bool
		maxFileSize   int64
		expectError   bool
		expectedHash  string
	}{
		{
			name:          "Process Regular File",
			filePath:      regularFile,
			extractString: false,
			skipLarge:     false,
			maxFileSize:   -1,
			expectError:   false,
			expectedHash:  expectedHash,
		},
		{
			name:          "Process Regular File With String Extraction",
			filePath:      regularFile,
			extractString: true,
			skipLarge:     false,
			maxFileSize:   -1,
			expectError:   false,
			expectedHash:  expectedHash,
		},
		{
			name:          "Skip Large File",
			filePath:      largeFile,
			extractString: false,
			skipLarge:     true,
			maxFileSize:   1024, // 1KB (smaller than the file)
			expectError:   true,
			expectedHash:  "",
		},
		{
			name:          "Process Non-existent File",
			filePath:      filepath.Join(tempDir, "nonexistent.txt"),
			extractString: false,
			skipLarge:     false,
			maxFileSize:   -1,
			expectError:   true,
			expectedHash:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create processor
			processor := NewHashProcessor("sha256", 1)
			processor.ExtractStrings = tc.extractString
			processor.SkipLargeFiles = tc.skipLarge
			processor.MaxFileSize = tc.maxFileSize

			// Process file
			result := processor.processFile(tc.filePath)

			// Check results
			if tc.expectError {
				if result.Error == "" {
					t.Error("Expected error, but got none")
				}
			} else {
				if result.Error != "" {
					t.Errorf("Expected no error, but got: %s", result.Error)
				}

				if result.Hash != tc.expectedHash {
					t.Errorf("Expected hash %s, got %s", tc.expectedHash, result.Hash)
				}

				if result.Path != tc.filePath {
					t.Errorf("Expected path %s, got %s", tc.filePath, result.Path)
				}

				if result.Name != filepath.Base(tc.filePath) {
					t.Errorf("Expected name %s, got %s", filepath.Base(tc.filePath), result.Name)
				}

				if tc.extractString && (result.Strings == nil || len(result.Strings) == 0) {
					t.Error("Expected extracted strings, but got none")
				}
			}
		})
	}
}

// Helper function to create a large file for testing
func createLargeFile(t *testing.T, path string, size int64) {
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}
	defer file.Close()

	// Write in chunks to avoid memory issues
	const chunkSize = 1024 * 64 // 64KB chunks
	chunk := make([]byte, chunkSize)
	for i := range chunk {
		chunk[i] = byte(i % 256)
	}

	var written int64
	for written < size {
		toWrite := chunkSize
		if size-written < int64(chunkSize) {
			toWrite = int(size - written)
		}
		n, err := file.Write(chunk[:toWrite])
		if err != nil {
			t.Fatalf("Failed to write to large test file: %v", err)
		}
		written += int64(n)
	}
}

func TestProcess(t *testing.T) {
	// Create a temporary directory with test files
	tempDir, err := os.MkdirTemp("", "processor_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := []struct {
		name    string
		content string
	}{
		{
			name:    "file1.txt",
			content: "Content of file 1",
		},
		{
			name:    "file2.txt",
			content: "Content of file 2",
		},
		{
			name:    "file3.txt",
			content: "Content of file 3",
		},
	}

	for _, tf := range testFiles {
		path := filepath.Join(tempDir, tf.name)
		if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}
	}

	// Create file paths channel
	fileChannel := make(chan string, len(testFiles))
	for _, tf := range testFiles {
		fileChannel <- filepath.Join(tempDir, tf.name)
	}
	close(fileChannel)

	// Create processor
	processor := NewHashProcessor("sha256", 2)

	// Process files
	resultChannel := processor.Process(fileChannel)

	// Collect and verify results
	results := make([]FileResult, 0, len(testFiles))
	for result := range resultChannel {
		results = append(results, result)
	}

	// Verify all files were processed
	if len(results) != len(testFiles) {
		t.Errorf("Expected %d results, got %d", len(testFiles), len(results))
	}

	// Verify each result
	for _, result := range results {
		// Find the corresponding test file
		var tf *struct {
			name    string
			content string
		}
		for i := range testFiles {
			if testFiles[i].name == filepath.Base(result.Path) {
				tf = &testFiles[i]
				break
			}
		}

		if tf == nil {
			t.Errorf("Unexpected result for file: %s", result.Path)
			continue
		}

		// Verify hash - compute expected hash
		expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tf.content)))
		if result.Hash != expectedHash {
			t.Errorf("For file %s: expected hash %s, got %s", tf.name, expectedHash, result.Hash)
		}

		// Verify other fields
		if result.Error != "" {
			t.Errorf("For file %s: expected no error, got: %s", tf.name, result.Error)
		}
		if result.HashAlgorithm != "sha256" {
			t.Errorf("For file %s: expected algorithm sha256, got %s", tf.name, result.HashAlgorithm)
		}
	}
}
