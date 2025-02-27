package dedup

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vtriple/agentflux/pkg/processor"
)

func TestDeduplicationEngine_Deduplicate(t *testing.T) {
	// Create sample file results
	now := time.Now()
	results := []processor.FileResult{
		{
			Path:          "/path/to/file1.txt",
			Name:          "file1.txt",
			HashAlgorithm: "sha256",
			Hash:          "abc123",
			Size:          100,
			ProcessedAt:   now,
		},
		{
			Path:          "/path/to/file2.txt",
			Name:          "file2.txt",
			HashAlgorithm: "sha256",
			Hash:          "def456",
			Size:          200,
			ProcessedAt:   now,
		},
		{
			Path:          "/other/path/to/file1_copy.txt",
			Name:          "file1_copy.txt",
			HashAlgorithm: "sha256",
			Hash:          "abc123", // Same hash as file1.txt
			Size:          100,
			ProcessedAt:   now,
		},
		{
			Path:          "/path/to/file3.txt",
			Name:          "file3.txt",
			HashAlgorithm: "sha256",
			Hash:          "ghi789",
			Size:          300,
			ProcessedAt:   now,
		},
		{
			Path:          "/path/with/error.txt",
			Name:          "error.txt",
			HashAlgorithm: "sha256",
			Hash:          "",
			Size:          0,
			Error:         "Error processing file",
			ProcessedAt:   now,
		},
	}

	// Test cases for different deduplication types
	tests := []struct {
		name           string
		dedupType      DeduplicationType
		expectedUnique int // Expected number of unique files
	}{
		{
			name:           "Hash Deduplication",
			dedupType:      HashDedup,
			expectedUnique: 3, // file1.txt, file2.txt, file3.txt (file1_copy.txt is a duplicate by hash)
		},
		{
			name:           "Path Deduplication",
			dedupType:      PathDedup,
			expectedUnique: 4, // All files have unique paths
		},
		{
			name:           "Name+Size Deduplication",
			dedupType:      NameDedup,
			expectedUnique: 4, // All files have unique names (or name+size combinations)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create engine with the test deduplication type
			engine := NewDeduplicationEngine(tc.dedupType)

			// Create input channel and context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			inputChan := make(chan processor.FileResult)
			outputChan := engine.Deduplicate(ctx, inputChan)

			// Create a WaitGroup to wait for the results
			var wg sync.WaitGroup
			wg.Add(1)

			// Start a goroutine to read results from the output channel
			uniqueResults := []processor.FileResult{}
			go func() {
				defer wg.Done()
				for result := range outputChan {
					uniqueResults = append(uniqueResults, result)
				}
			}()

			// Send all test results to the input channel
			for _, result := range results {
				inputChan <- result
			}
			close(inputChan)

			// Wait for all results to be processed
			wg.Wait()

			// Check stats
			total, unique := engine.GetStats()
			if total != len(results) {
				t.Errorf("Expected total count %d, got %d", len(results), total)
			}

			if unique != tc.expectedUnique {
				t.Errorf("Expected unique count %d, got %d", tc.expectedUnique, unique)
			}

			// Check that the number of results in the output channel matches the expected count
			// Note: Files with errors are not passed through in the tests
			expectedOutputCount := tc.expectedUnique
			if len(uniqueResults) != expectedOutputCount {
				t.Errorf("Expected %d results in output channel, got %d", expectedOutputCount, len(uniqueResults))
			}
		})
	}
}

func TestDeduplicationEngine_GetDeduplicationKey(t *testing.T) {
	// Create a sample file result
	result := processor.FileResult{
		Path:          "/path/to/file.txt",
		Name:          "file.txt",
		HashAlgorithm: "sha256",
		Hash:          "abc123",
		Size:          100,
	}

	// Test cases
	tests := []struct {
		name          string
		dedupType     DeduplicationType
		expectedKey   string
	}{
		{
			name:          "Hash Deduplication",
			dedupType:     HashDedup,
			expectedKey:   "sha256:abc123",
		},
		{
			name:          "Path Deduplication",
			dedupType:     PathDedup,
			expectedKey:   "/path/to/file.txt",
		},
		{
			name:          "Name Deduplication",
			dedupType:     NameDedup,
			expectedKey:   "file.txt#100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := NewDeduplicationEngine(tc.dedupType)
			key := engine.getDeduplicationKey(result)
			if key != tc.expectedKey {
				t.Errorf("Expected key %s, got %s", tc.expectedKey, key)
			}
		})
	}
}

func TestDeduplicationEngine_Reset(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Add data directly to the engine to avoid race conditions
	engine.lock.Lock()
	engine.totalFiles = 2
	engine.uniqueFiles = 2
	engine.seen["sha256:abc123"] = true
	engine.seen["sha256:def456"] = true
	engine.lock.Unlock()
	
	// Check stats before reset
	total, unique := engine.GetStats()
	if total != 2 {
		t.Errorf("Expected total count 2, got %d", total)
	}
	if unique != 2 {
		t.Errorf("Expected unique count 2, got %d", unique)
	}
	
	// Reset the engine
	engine.Reset()
	
	// Check stats after reset
	total, unique = engine.GetStats()
	if total != 0 {
		t.Errorf("Expected total count 0 after reset, got %d", total)
	}
	if unique != 0 {
		t.Errorf("Expected unique count 0 after reset, got %d", unique)
	}
}

// TestDeduplicate_WithErrorsOnly tests handling of files with errors only
func TestDeduplicate_WithErrorsOnly(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Create a channel with error results only
	inputChan := make(chan processor.FileResult, 3)
	inputChan <- processor.FileResult{
		Path:  "/path/to/error1.txt",
		Name:  "error1.txt",
		Error: "File not found",
	}
	inputChan <- processor.FileResult{
		Path:  "/path/to/error2.txt",
		Name:  "error2.txt",
		Error: "Permission denied",
	}
	inputChan <- processor.FileResult{
		Path:  "/path/to/error3.txt",
		Name:  "error3.txt",
		Error: "I/O error",
	}
	close(inputChan)
	
	// Run deduplication
	ctx := context.Background()
	outputChan := engine.Deduplicate(ctx, inputChan)
	
	// Count results
	resultCount := 0
	for range outputChan {
		resultCount++
	}
	
	// Should have no results since error files are skipped
	if resultCount != 0 {
		t.Errorf("Expected 0 results (all errors), got %d", resultCount)
	}
	
	// Stats should count total files but no unique files
	total, unique := engine.GetStats()
	if total != 3 {
		t.Errorf("Expected total count 3, got %d", total)
	}
	if unique != 0 {
		t.Errorf("Expected unique count 0, got %d", unique)
	}
}

// TestDeduplicate_WithNilChannel tests handling of nil input channel
func TestDeduplicate_WithNilChannel(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Run deduplication with nil channel
	ctx := context.Background()
	outputChan := engine.Deduplicate(ctx, nil)
	
	// Count results
	resultCount := 0
	for range outputChan {
		resultCount++
	}
	
	// Should have no results
	if resultCount != 0 {
		t.Errorf("Expected 0 results, got %d", resultCount)
	}
}

// TestDeduplicate_WithCancelledContext tests handling of cancelled context
func TestDeduplicate_WithCancelledContext(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	// Create input channel
	inputChan := make(chan processor.FileResult, 10)
	for i := 0; i < 10; i++ {
		inputChan <- processor.FileResult{
			Path:          "/path/to/file" + string(rune('0'+i)) + ".txt",
			Name:          "file" + string(rune('0'+i)) + ".txt",
			Hash:          "hash" + string(rune('0'+i)),
			HashAlgorithm: "sha256",
		}
	}
	// Don't close the channel - should not matter as context is cancelled
	
	// Run deduplication
	outputChan := engine.Deduplicate(ctx, inputChan)
	
	// Count results
	resultCount := 0
	for range outputChan {
		resultCount++
	}
	
	// Should have no results due to cancelled context
	if resultCount != 0 {
		t.Errorf("Expected 0 results due to cancelled context, got %d", resultCount)
	}
}

// TestDeduplicate_ConcurrencyHandling tests handling concurrent operations
func TestDeduplicate_ConcurrencyHandling(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Create a large input channel
	inputChan := make(chan processor.FileResult, 1000)
	for i := 0; i < 1000; i++ {
		// Create groups of 10 files with the same hash to test deduplication
		inputChan <- processor.FileResult{
			Path:          "/path/to/file" + string(rune('0'+i%10)) + "_" + string(rune('0'+(i/10)%10)) + ".txt",
			Name:          "file" + string(rune('0'+i%10)) + ".txt",
			Hash:          "hash" + string(rune('0'+i%10)), // Same hash for every 10 files
			HashAlgorithm: "sha256",
		}
	}
	close(inputChan)
	
	// Run deduplication with concurrent operations
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	outputChan := engine.Deduplicate(ctx, inputChan)
	
	// Start a goroutine to read stats while deduplication is running
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			total, unique := engine.GetStats()
			// Just read stats - we don't validate the values as they'll be changing
			if total < 0 || unique < 0 {
				t.Errorf("Invalid stats: total=%d, unique=%d", total, unique)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	
	// Count results and verify we only get unique files
	seenHashes := make(map[string]bool)
	resultCount := 0
	
	for result := range outputChan {
		resultCount++
		hashKey := result.HashAlgorithm + ":" + result.Hash
		
		if seenHashes[hashKey] {
			t.Errorf("Duplicate hash found: %s", hashKey)
		}
		seenHashes[hashKey] = true
	}
	
	// Wait for stats reading to complete
	wg.Wait()
	
	// Should have 10 unique results (one for each hash)
	if resultCount != 10 {
		t.Errorf("Expected 10 unique results, got %d", resultCount)
	}
	
	// Stats should show 1000 total files and 10 unique
	total, unique := engine.GetStats()
	if total != 1000 {
		t.Errorf("Expected total count 1000, got %d", total)
	}
	if unique != 10 {
		t.Errorf("Expected unique count 10, got %d", unique)
	}
}
