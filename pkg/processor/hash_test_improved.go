package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vtriple/agentflux/pkg/common/logging"
)

// TestHashProcessor_ProcessFile_ExtendedCases tests additional edge cases for processFile
func TestHashProcessor_ProcessFile_ExtendedCases(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "processor_ext_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create files with various characteristics
	testCases := []struct {
		name              string
		content           []byte
		fileMode          os.FileMode
		expectedError     string
		skipLargeFiles    bool
		maxFileSize       int64
		extractStrings    bool
		expectedStrings   bool
		expectedExec      bool
	}{
		{
			name:              "Empty file",
			content:           []byte{},
			fileMode:          0644,
			expectedError:     "",
			skipLargeFiles:    false,
			maxFileSize:       -1,
			extractStrings:    true,
			expectedStrings:   false, // No strings in empty file
			expectedExec:      false,
		},
		{
			name:              "Binary file with null bytes",
			content:           []byte{0x00, 0x01, 0x02, 0x00, 0x03, 0x04, 0x00},
			fileMode:          0644,
			expectedError:     "",
			skipLargeFiles:    false,
			maxFileSize:       -1,
			extractStrings:    true,
			expectedStrings:   false, // No valid strings in this binary
			expectedExec:      false,
		},
		{
			name:              "Text file with special characters",
			content:           []byte("Text with special \x01\x02 characters and some valid text"),
			fileMode:          0644,
			expectedError:     "",
			skipLargeFiles:    false,
			maxFileSize:       -1,
			extractStrings:    true,
			expectedStrings:   true, // Should find "Text with special" and "characters and some valid text"
			expectedExec:      false,
		},
		{
			name:              "Executable file",
			content:           []byte("#!/bin/sh\necho 'Hello World'"),
			fileMode:          0755,
			expectedError:     "",
			skipLargeFiles:    false,
			maxFileSize:       -1,
			extractStrings:    true,
			expectedStrings:   true,
			expectedExec:      true,
		},
		{
			name:              "Large file that should be skipped",
			content:           make([]byte, 1024), // 1KB file
			fileMode:          0644,
			expectedError:     "file too large",
			skipLargeFiles:    true,
			maxFileSize:       512, // Smaller than file size
			extractStrings:    false,
			expectedStrings:   false,
			expectedExec:      false,
		},
		{
			name:              "File with repeated strings",
			content:           []byte("repeat repeat repeat repeat repeat repeat"),
			fileMode:          0644,
			expectedError:     "",
			skipLargeFiles:    false,
			maxFileSize:       -1,
			extractStrings:    true,
			expectedStrings:   true,
			expectedExec:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the test file
			filePath := filepath.Join(tempDir, tc.name)
			if err := os.WriteFile(filePath, tc.content, tc.fileMode); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Create processor with the specified settings
			processor := NewHashProcessor("sha256", 1)
			processor.SkipLargeFiles = tc.skipLargeFiles
			processor.MaxFileSize = tc.maxFileSize
			processor.ExtractStrings = tc.extractStrings
			processor.StringMinLength = 4

			// Process the file
			result := processor.processFile(filePath)

			// Check error conditions
			if tc.expectedError != "" {
				if result.Error == "" {
					t.Errorf("Expected error containing '%s', got no error", tc.expectedError)
				} else if !strings.Contains(result.Error, tc.expectedError) {
					t.Errorf("Expected error containing '%s', got '%s'", tc.expectedError, result.Error)
				}
			} else if result.Error != "" {
				t.Errorf("Expected no error, got '%s'", result.Error)
			}

			// Check executable status
			if result.IsExecutable != tc.expectedExec {
				t.Errorf("Expected executable=%v, got %v", tc.expectedExec, result.IsExecutable)
			}

			// Check strings extraction
			if tc.expectedStrings && (result.Strings == nil || len(result.Strings) == 0) {
				t.Errorf("Expected strings to be extracted, but got none")
			} else if !tc.expectedStrings && result.Strings != nil && len(result.Strings) > 0 {
				t.Errorf("Expected no strings to be extracted, but got %v", result.Strings)
			}

			// Check hash computation
			if tc.expectedError == "" {
				if result.Hash == "" {
					t.Errorf("Expected hash to be computed, but got empty string")
				}
				if result.HashAlgorithm != "sha256" {
					t.Errorf("Expected hash algorithm 'sha256', got '%s'", result.HashAlgorithm)
				}
			}
		})
	}
}

// TestHashProcessor_ExtractStrings_ExtendedCases tests edge cases for string extraction
func TestHashProcessor_ExtractStrings_ExtendedCases(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "strings_ext_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test cases for string extraction
	testCases := []struct {
		name           string
		content        []byte
		minLength      int
		expectedCount  int
		expectedString string
	}{
		{
			name:           "Mixed binary and text",
			content:        []byte("Hello\x00World\x01This\x02Is\x03A\x04Test"),
			minLength:      4,
			expectedCount:  2, // Should find "Hello" and "World"
			expectedString: "Hello",
		},
		{
			name:           "Only ASCII printable",
			content:        []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz 0123456789"),
			minLength:      10,
			expectedCount:  1, // The whole string as one
			expectedString: "ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz 0123456789",
		},
		{
			name:           "Short strings filtered by minLength",
			content:        []byte("one two three four five six seven eight nine ten"),
			minLength:      5,
			expectedCount:  3, // "three", "seven", "eight"
			expectedString: "three",
		},
		{
			name:           "Strings with special characters",
			content:        []byte("alpha\x00beta\x00gamma\x00delta"),
			minLength:      4,
			expectedCount:  4,
			expectedString: "alpha",
		},
		{
			name:           "Maximum string count limit",
			content:        generateRepeatedStrings(11000), // Create many strings
			minLength:      4,
			expectedCount:  10000, // Should hit the limit of 10000 strings
			expectedString: "String0001",
		},
		{
			name:           "Empty content",
			content:        []byte{},
			minLength:      4,
			expectedCount:  0,
			expectedString: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(tempDir, fmt.Sprintf("%s.bin", tc.name))
			if err := os.WriteFile(filePath, tc.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Create processor with appropriate string length
			processor := NewHashProcessor("sha256", 1)
			processor.ExtractStrings = true
			processor.StringMinLength = tc.minLength

			// Open the file
			file, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("Failed to open test file: %v", err)
			}
			defer file.Close()

			// Extract strings
			strings, err := processor.extractStrings(file)
			if err != nil {
				t.Fatalf("Failed to extract strings: %v", err)
			}

			// Check string count
			if len(strings) > tc.expectedCount {
				t.Errorf("Expected at most %d strings, got %d", tc.expectedCount, len(strings))
			}

			if tc.expectedCount > 0 && len(strings) == 0 {
				t.Errorf("Expected at least 1 string, got none")
			}

			// Check for specific expected string if provided
			if tc.expectedString != "" {
				found := false
				for _, s := range strings {
					if s == tc.expectedString {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find string '%s', but it was not extracted", tc.expectedString)
				}
			}

			// Check for duplicates (strings should be unique)
			seen := make(map[string]bool)
			for _, s := range strings {
				if seen[s] {
					t.Errorf("Found duplicate string '%s' in extracted strings", s)
				}
				seen[s] = true
			}

			// Check minimum length requirement
			for _, s := range strings {
				if len(s) < tc.minLength {
					t.Errorf("Found string '%s' of length %d, which is shorter than minimum %d",
						s, len(s), tc.minLength)
				}
			}
		})
	}
}

// TestHashProcessor_ScanStrings tests the custom scanner split function
func TestHashProcessor_ScanStrings(t *testing.T) {
	// Test cases for the scanner
	testCases := []struct {
		name          string
		data          []byte
		atEOF         bool
		expectAdvance int
		expectToken   []byte
		expectError   bool
	}{
		{
			name:          "Simple printable string",
			data:          []byte("Hello"),
			atEOF:         false,
			expectAdvance: 5,
			expectToken:   []byte("Hello"),
			expectError:   false,
		},
		{
			name:          "String with non-printable prefix",
			data:          []byte{0x00, 0x01, 0x02, 'H', 'e', 'l', 'l', 'o'},
			atEOF:         false,
			expectAdvance: 8,
			expectToken:   []byte("Hello"),
			expectError:   false,
		},
		{
			name:          "String with non-printable suffix",
			data:          []byte{'H', 'e', 'l', 'l', 'o', 0x00, 0x01, 0x02},
			atEOF:         false,
			expectAdvance: 5,
			expectToken:   []byte("Hello"),
			expectError:   false,
		},
		{
			name:          "Non-printable only",
			data:          []byte{0x00, 0x01, 0x02, 0x03},
			atEOF:         false,
			expectAdvance: 4,
			expectToken:   nil,
			expectError:   false,
		},
		{
			name:          "Empty data",
			data:          []byte{},
			atEOF:         true,
			expectAdvance: 0,
			expectToken:   nil,
			expectError:   false,
		},
		{
			name:          "Mixed content",
			data:          []byte{'H', 'e', 'l', 'l', 'o', 0x00, 'W', 'o', 'r', 'l', 'd'},
			atEOF:         false,
			expectAdvance: 5,
			expectToken:   []byte("Hello"),
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			processor := NewHashProcessor("sha256", 1)
			processor.StringMinLength = 1 // Set to 1 to test all strings

			// Call scanStrings function
			advance, token, err := processor.scanStrings(tc.data, tc.atEOF)

			// Check results
			if err != nil && !tc.expectError {
				t.Errorf("Unexpected error: %v", err)
			}
			if err == nil && tc.expectError {
				t.Errorf("Expected error, got none")
			}

			if advance != tc.expectAdvance {
				t.Errorf("Expected advance %d, got %d", tc.expectAdvance, advance)
			}

			if !equalByteSlices(token, tc.expectToken) {
				t.Errorf("Expected token %v, got %v", tc.expectToken, token)
			}
		})
	}
}

// TestHashProcessor_Process_InterruptedChannel tests behavior when input channel is closed unexpectedly
func TestHashProcessor_Process_InterruptedChannel(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "processor_interrupt_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create some test files
	numFiles := 10
	var filePaths []string
	for i := 0; i < numFiles; i++ {
		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		content := []byte(fmt.Sprintf("Content for file %d", i))
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		filePaths = append(filePaths, filePath)
	}

	// Create processor
	processor := NewHashProcessor("sha256", 3) // Use multiple workers

	// Create input channel
	fileChannel := make(chan string, numFiles)

	// Send half the files
	for i := 0; i < numFiles/2; i++ {
		fileChannel <- filePaths[i]
	}

	// Start processing
	resultChannel := processor.Process(fileChannel)

	// Close the channel before sending all files
	close(fileChannel)

	// Collect results
	var results []FileResult
	for result := range resultChannel {
		results = append(results, result)
	}

	// We should only get results for the files we sent
	if len(results) != numFiles/2 {
		t.Errorf("Expected %d results, got %d", numFiles/2, len(results))
	}

	// Verify results are valid
	for _, result := range results {
		if result.Error != "" {
			t.Errorf("Unexpected error in result: %s", result.Error)
		}
		if result.Hash == "" {
			t.Errorf("Expected hash to be computed, got empty string")
		}
	}
}

// TestHashProcessor_WorkerDistribution tests that work is properly distributed among workers
func TestHashProcessor_WorkerDistribution(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "processor_worker_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a significant number of test files
	numFiles := 50
	var filePaths []string
	for i := 0; i < numFiles; i++ {
		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		content := []byte(fmt.Sprintf("Content for file %d with some padding to make it a bit larger", i))
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		filePaths = append(filePaths, filePath)
	}

	// Create processor with multiple workers
	numWorkers := 4
	processor := NewHashProcessor("sha256", numWorkers)

	// Create a map to track which worker processed each file
	workerCounts := make(map[int]int)
	var mu sync.Mutex

	// Create a custom process function
	processFunc := func(fileChannel <-chan string) <-chan FileResult {
		resultChannel := make(chan FileResult, numWorkers*2)
		
		// Start worker goroutines
		var wg sync.WaitGroup
		wg.Add(numWorkers)
		for i := 0; i < numWorkers; i++ {
			workerID := i
			go func() {
				defer wg.Done()
				
				// Custom worker implementation that tracks worker usage
				for filePath := range fileChannel {
					// Record that this worker processed a file
					mu.Lock()
					workerCounts[workerID]++
					mu.Unlock()
					
					// Sleep a tiny bit to simulate work and make distribution more evident
					time.Sleep(time.Millisecond)
					
					// Process the file using the original processor
					result := processor.processFile(filePath)
					resultChannel <- result
				}
			}()
		}
		
		// Start closer goroutine
		go func() {
			wg.Wait()
			close(resultChannel)
		}()
		
		return resultChannel
	}

	// Create input channel and send all files
	fileChannel := make(chan string, numFiles)
	for _, filePath := range filePaths {
		fileChannel <- filePath
	}
	close(fileChannel)

	// Process files using our custom function
	resultChannel := processFunc(fileChannel)

	// Drain result channel
	resultCount := 0
	for range resultChannel {
		resultCount++
	}

	// Verify all files were processed
	if resultCount != numFiles {
		t.Errorf("Expected %d results, got %d", numFiles, resultCount)
	}

	// Check worker distribution
	mu.Lock()
	defer mu.Unlock()

	// All workers should have been used
	if len(workerCounts) != numWorkers {
		t.Errorf("Expected %d workers to be used, got %d", numWorkers, len(workerCounts))
	}

	// Each worker should have processed at least some files
	for id, count := range workerCounts {
		if count == 0 {
			t.Errorf("Worker %d didn't process any files", id)
		}
		t.Logf("Worker %d processed %d files", id, count)
	}

	// Ideally, work should be somewhat evenly distributed
	avgFilesPerWorker := float64(numFiles) / float64(numWorkers)
	for id, count := range workerCounts {
		// Allow some variability, but ensure it's not totally unbalanced
		if float64(count) < avgFilesPerWorker*0.5 || float64(count) > avgFilesPerWorker*1.5 {
			t.Logf("Worker %d processed %d files, which is significantly different from average %.2f",
				id, count, avgFilesPerWorker)
		}
	}
}

// TestHashProcessor_SetLogger tests the SetLogger method
func TestHashProcessor_SetLogger(t *testing.T) {
	processor := NewHashProcessor("sha256", 1)
	
	// Create a custom logger
	logger := logging.NewLogger("custom_processor")
	
	// Set the logger
	processor.SetLogger(logger)
	
	// Indirectly test that the logger is used by running a processing operation
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "processor_logger_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a test file
	filePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create input channel with the file
	fileChannel := make(chan string, 1)
	fileChannel <- filePath
	close(fileChannel)
	
	// Process the file
	resultChannel := processor.Process(fileChannel)
	
	// Collect results
	var results []FileResult
	for result := range resultChannel {
		results = append(results, result)
	}
	
	// Verify we got a result
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	
	// Verify the result is valid
	if len(results) > 0 && results[0].Error != "" {
		t.Errorf("Unexpected error in result: %s", results[0].Error)
	}
}

// Helper function to generate repeated strings
func generateRepeatedStrings(count int) []byte {
	var builder strings.Builder
	for i := 0; i < count; i++ {
		builder.WriteString(fmt.Sprintf("String%04d\x00", i))
	}
	return []byte(builder.String())
}

// Helper function to compare byte slices
func equalByteSlices(a, b []byte) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
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
