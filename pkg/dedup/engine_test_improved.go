package dedup

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/vtriple/agentflux/pkg/common/logging"
	"github.com/vtriple/agentflux/pkg/processor"
)

// TestDeduplicationEngine_ExtendedDedupTypes tests all deduplication types thoroughly
func TestDeduplicationEngine_ExtendedDedupTypes(t *testing.T) {
	// Test cases for all deduplication types
	tests := []struct {
		name                  string
		dedupType             DeduplicationType
		inputResults          []processor.FileResult
		expectedUniqueCount   int
		expectedDuplicateKeys []string
	}{
		{
			name:      "HashDedup_IdenticalHashes",
			dedupType: HashDedup,
			inputResults: []processor.FileResult{
				{Path: "/path1/file1.txt", Name: "file1.txt", Size: 100, Hash: "abc123", HashAlgorithm: "sha256"},
				{Path: "/path2/file2.txt", Name: "file2.txt", Size: 200, Hash: "abc123", HashAlgorithm: "sha256"}, // Same hash
				{Path: "/path3/file3.txt", Name: "file3.txt", Size: 300, Hash: "def456", HashAlgorithm: "sha256"},
			},
			expectedUniqueCount:   2, // Only 2 unique hashes
			expectedDuplicateKeys: []string{"sha256:abc123"},
		},
		{
			name:      "HashDedup_DifferentAlgorithms",
			dedupType: HashDedup,
			inputResults: []processor.FileResult{
				{Path: "/path1/file1.txt", Name: "file1.txt", Size: 100, Hash: "abc123", HashAlgorithm: "sha256"},
				{Path: "/path2/file2.txt", Name: "file2.txt", Size: 100, Hash: "abc123", HashAlgorithm: "md5"}, // Different algo
				{Path: "/path3/file3.txt", Name: "file3.txt", Size: 100, Hash: "def456", HashAlgorithm: "sha256"},
			},
			expectedUniqueCount: 3, // Different hash algorithms treated as unique
			expectedDuplicateKeys: []string{},
		},
		{
			name:      "PathDedup_IdenticalPaths",
			dedupType: PathDedup,
			inputResults: []processor.FileResult{
				{Path: "/path1/file.txt", Name: "file.txt", Size: 100, Hash: "abc123", HashAlgorithm: "sha256"},
				{Path: "/path1/file.txt", Name: "file.txt", Size: 200, Hash: "def456", HashAlgorithm: "sha256"}, // Same path
				{Path: "/path2/file.txt", Name: "file.txt", Size: 300, Hash: "ghi789", HashAlgorithm: "sha256"},
			},
			expectedUniqueCount:   2, // Only 2 unique paths
			expectedDuplicateKeys: []string{"/path1/file.txt"},
		},
		{
			name:      "NameDedup_IdenticalNamesAndSizes",
			dedupType: NameDedup,
			inputResults: []processor.FileResult{
				{Path: "/path1/file.txt", Name: "file.txt", Size: 100, Hash: "abc123", HashAlgorithm: "sha256"},
				{Path: "/path2/file.txt", Name: "file.txt", Size: 100, Hash: "def456", HashAlgorithm: "sha256"}, // Same name+size
				{Path: "/path3/file.txt", Name: "file.txt", Size: 200, Hash: "ghi789", HashAlgorithm: "sha256"}, // Different size
			},
			expectedUniqueCount:   2, // 2 unique name+size combos
			expectedDuplicateKeys: []string{"file.txt#100"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create engine with test type
			engine := NewDeduplicationEngine(tc.dedupType)
			
			// Create test input channel
			inputCh := make(chan processor.FileResult, len(tc.inputResults))
			for _, result := range tc.inputResults {
				inputCh <- result
			}
			close(inputCh)
			
			// Process with the engine
			ctx := context.Background()
			outputCh := engine.Deduplicate(ctx, inputCh)
			
			// Collect outputs
			var outputResults []processor.FileResult
			for result := range outputCh {
				outputResults = append(outputResults, result)
			}
			
			// Check stats
			total, unique := engine.GetStats()
			if total != len(tc.inputResults) {
				t.Errorf("Expected total count %d, got %d", len(tc.inputResults), total)
			}
			if unique != tc.expectedUniqueCount {
				t.Errorf("Expected unique count %d, got %d", tc.expectedUniqueCount, unique)
			}
			
			// Verify output length
			if len(outputResults) != tc.expectedUniqueCount {
				t.Errorf("Expected %d unique results, got %d", tc.expectedUniqueCount, len(outputResults))
			}
			
			// Check duplicates
			for _, key := range tc.expectedDuplicateKeys {
				count := 0
				for _, result := range tc.inputResults {
					if engine.getDeduplicationKey(result) == key {
						count++
					}
				}
				if count <= 1 {
					t.Errorf("Expected key %s to have multiple occurrences", key)
				}
			}
		})
	}
}

// TestDeduplicationEngine_ContextCancellation tests behavior when context is cancelled
func TestDeduplicationEngine_ContextCancellation(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create input channel 
	inputCh := make(chan processor.FileResult, 100)
	
	// Start deduplication
	outputCh := engine.Deduplicate(ctx, inputCh)
	
	// Send a few results
	for i := 0; i < 5; i++ {
		inputCh <- processor.FileResult{
			Path:          fmt.Sprintf("/path/file%d.txt", i),
			Name:          fmt.Sprintf("file%d.txt", i),
			Size:          int64(100 * i),
			Hash:          fmt.Sprintf("hash%d", i),
			HashAlgorithm: "sha256",
		}
	}
	
	// Cancel context
	cancel()
	
	// Send a few more results
	for i := 5; i < 10; i++ {
		inputCh <- processor.FileResult{
			Path:          fmt.Sprintf("/path/file%d.txt", i),
			Name:          fmt.Sprintf("file%d.txt", i),
			Size:          int64(100 * i),
			Hash:          fmt.Sprintf("hash%d", i),
			HashAlgorithm: "sha256",
		}
	}
	
	// Close the input channel
	close(inputCh)
	
	// Collect all results
	var results []processor.FileResult
	for result := range outputCh {
		results = append(results, result)
	}
	
	// We expect processing to have stopped due to context cancellation
	// So we should only get the results sent before cancellation
	if len(results) > 5 {
		t.Errorf("Expected at most 5 results after context cancellation, got %d", len(results))
	}
}

// TestDeduplicationEngine_ExtendedReset tests the Reset functionality more thoroughly
func TestDeduplicationEngine_ExtendedReset(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Create input with duplicates
	inputResults := []processor.FileResult{
		{Path: "/path1/file1.txt", Name: "file1.txt", Size: 100, Hash: "abc123", HashAlgorithm: "sha256"},
		{Path: "/path2/file2.txt", Name: "file2.txt", Size: 200, Hash: "abc123", HashAlgorithm: "sha256"}, // Same hash
		{Path: "/path3/file3.txt", Name: "file3.txt", Size: 300, Hash: "def456", HashAlgorithm: "sha256"},
	}
	
	// Process first batch
	inputCh := make(chan processor.FileResult, len(inputResults))
	for _, result := range inputResults {
		inputCh <- result
	}
	close(inputCh)
	
	ctx := context.Background()
	outputCh := engine.Deduplicate(ctx, inputCh)
	
	// Collect and count first batch
	count1 := 0
	for range outputCh {
		count1++
	}
	
	// Verify first batch results
	total1, unique1 := engine.GetStats()
	if total1 != 3 || unique1 != 2 {
		t.Errorf("First batch: expected stats total=3, unique=2, got total=%d, unique=%d", 
			total1, unique1)
	}
	
	// Reset the engine
	engine.Reset()
	
	// Verify stats were reset
	total2, unique2 := engine.GetStats()
	if total2 != 0 || unique2 != 0 {
		t.Errorf("After reset: expected stats total=0, unique=0, got total=%d, unique=%d", 
			total2, unique2)
	}
	
	// Process same batch again
	inputCh = make(chan processor.FileResult, len(inputResults))
	for _, result := range inputResults {
		inputCh <- result
	}
	close(inputCh)
	
	outputCh = engine.Deduplicate(ctx, inputCh)
	
	// Collect and count second batch
	count2 := 0
	for range outputCh {
		count2++
	}
	
	// After reset, we should see the same deduplication results as the first time
	if count2 != count1 {
		t.Errorf("Expected same number of results after reset: %d, got %d", count1, count2)
	}
	
	// Stats should be the same as first run
	total3, unique3 := engine.GetStats()
	if total3 != total1 || unique3 != unique1 {
		t.Errorf("After reset and second batch: expected stats total=%d, unique=%d, got total=%d, unique=%d",
			total1, unique1, total3, unique3)
	}
}

// TestDeduplicationEngine_ErrorHandling tests handling of errors in input data
func TestDeduplicationEngine_ErrorHandling(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Create input with error results and normal results
	inputResults := []processor.FileResult{
		{Path: "/path1/error1.txt", Name: "error1.txt", Error: "File not found", Hash: "", HashAlgorithm: ""},
		{Path: "/path2/file2.txt", Name: "file2.txt", Size: 200, Hash: "abc123", HashAlgorithm: "sha256"},
		{Path: "/path3/error3.txt", Name: "error3.txt", Error: "Permission denied", Hash: "", HashAlgorithm: ""},
		{Path: "/path4/file4.txt", Name: "file4.txt", Size: 300, Hash: "def456", HashAlgorithm: "sha256"},
	}
	
	// Process batch
	inputCh := make(chan processor.FileResult, len(inputResults))
	for _, result := range inputResults {
		inputCh <- result
	}
	close(inputCh)
	
	ctx := context.Background()
	outputCh := engine.Deduplicate(ctx, inputCh)
	
	// Collect outputs
	var outputResults []processor.FileResult
	for result := range outputCh {
		outputResults = append(outputResults, result)
	}
	
	// Check results - error results should be filtered out
	total, unique := engine.GetStats()
	if len(outputResults) != 2 {
		t.Errorf("Expected 2 results (errors filtered), got %d", len(outputResults))
	}
	
	// Stats should count the total files including errors
	if total != 4 {
		t.Errorf("Expected total count 4 (including errors), got %d", total)
	}
	
	// Stats should only count unique non-error files
	if unique != 2 {
		t.Errorf("Expected unique count 2 (excluding errors), got %d", unique)
	}
	
	// Verify only non-error results are in output
	for _, result := range outputResults {
		if result.Error != "" {
			t.Errorf("Expected error results to be filtered, got result with error: %s", result.Error)
		}
	}
}

// TestDeduplicationEngine_ConcurrentAccess tests concurrent access to the engine
func TestDeduplicationEngine_ConcurrentAccess(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Create many goroutines that will access the engine concurrently
	var wg sync.WaitGroup
	iterations := 5
	batchSize := 10
	
	// Function to process a batch
	processBatch := func(id int) {
		defer wg.Done()
		
		// Create unique results for this batch
		results := make([]processor.FileResult, batchSize)
		for i := 0; i < batchSize; i++ {
			results[i] = processor.FileResult{
				Path:          fmt.Sprintf("/path%d/file%d.txt", id, i),
				Name:          fmt.Sprintf("file%d_%d.txt", id, i),
				Size:          int64((id * 100) + i),
				Hash:          fmt.Sprintf("hash%d_%d", id, i),
				HashAlgorithm: "sha256",
			}
		}
		
		// Create input channel
		inputCh := make(chan processor.FileResult, len(results))
		for _, result := range results {
			inputCh <- result
		}
		close(inputCh)
		
		// Process batch
		ctx := context.Background()
		outputCh := engine.Deduplicate(ctx, inputCh)
		
		// Drain output channel
		for range outputCh {
			// Just consume the results
		}
	}
	
	// Start goroutines
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go processBatch(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// Verify stats
	total, unique := engine.GetStats()
	expectedTotal := iterations * batchSize
	if total != expectedTotal {
		t.Errorf("Expected total count %d, got %d", expectedTotal, total)
	}
	
	// All results should be unique
	if unique != expectedTotal {
		t.Errorf("Expected unique count %d, got %d", expectedTotal, unique)
	}
}

// TestDeduplicationEngine_SetLogger tests the SetLogger method
func TestDeduplicationEngine_SetLogger(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Create a custom logger
	logger := logging.NewLogger("custom_dedup")
	
	// Set the logger
	engine.SetLogger(logger)
	
	// Test that the logger is used (indirectly by running a deduplication)
	inputCh := make(chan processor.FileResult, 1)
	inputCh <- processor.FileResult{
		Path:          "/path/file.txt",
		Name:          "file.txt",
		Size:          100,
		Hash:          "abc123",
		HashAlgorithm: "sha256",
	}
	close(inputCh)
	
	ctx := context.Background()
	outputCh := engine.Deduplicate(ctx, inputCh)
	
	// Just drain the output
	for range outputCh {
	}
	
	// Can't directly verify the logger was used, but we can check it didn't crash
	total, unique := engine.GetStats()
	if total != 1 || unique != 1 {
		t.Errorf("Expected 1 file processed, got total=%d, unique=%d", total, unique)
	}
}

// TestDeduplicationEngine_DefaultType tests that an unknown deduplication type defaults to hash deduplication
func TestDeduplicationEngine_DefaultType(t *testing.T) {
	// Create engine with unknown type
	engine := NewDeduplicationEngine("unknown_type")
	
	// Create test results with same hash
	inputResults := []processor.FileResult{
		{Path: "/path1/file1.txt", Name: "file1.txt", Size: 100, Hash: "same_hash", HashAlgorithm: "sha256"},
		{Path: "/path2/file2.txt", Name: "file2.txt", Size: 200, Hash: "same_hash", HashAlgorithm: "sha256"},
	}
	
	// Process batch
	inputCh := make(chan processor.FileResult, len(inputResults))
	for _, result := range inputResults {
		inputCh <- result
	}
	close(inputCh)
	
	ctx := context.Background()
	outputCh := engine.Deduplicate(ctx, inputCh)
	
	// Count results
	count := 0
	for range outputCh {
		count++
	}
	
	// If defaulting to hash deduplication, should get only 1 result
	if count != 1 {
		t.Errorf("Expected 1 result when defaulting to hash deduplication, got %d", count)
	}
	
	// Check stats
	total, unique := engine.GetStats()
	if total != 2 || unique != 1 {
		t.Errorf("Expected stats total=2, unique=1, got total=%d, unique=%d", total, unique)
	}
}

// TestDeduplicationEngine_RaceConditions tests for race conditions in the engine
func TestDeduplicationEngine_RaceConditions(t *testing.T) {
	// This test is designed to be run with -race flag to detect race conditions
	
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)
	
	// Create a context that will be used for deduplication
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create input channel
	inputCh := make(chan processor.FileResult, 100)
	
	// Start deduplication in a goroutine
	outputCh := engine.Deduplicate(ctx, inputCh)
	
	// Start a goroutine to consume results
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range outputCh {
			// Just consume results
		}
	}()
	
	// Start goroutines to call various methods concurrently
	wg.Add(4)
	
	// Goroutine to send data
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			inputCh <- processor.FileResult{
				Path:          fmt.Sprintf("/path/file%d.txt", i),
				Name:          fmt.Sprintf("file%d.txt", i),
				Size:          100,
				Hash:          fmt.Sprintf("hash%d", i%10), // Create some duplicates
				HashAlgorithm: "sha256",
			}
			// Introduce small random delays
			time.Sleep(time.Millisecond)
		}
		close(inputCh)
	}()
	
	// Goroutine to call GetStats repeatedly
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			engine.GetStats()
			time.Sleep(time.Millisecond)
		}
	}()
	
	// Goroutine to call SetLogger repeatedly
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			engine.SetLogger(logging.NewLogger(fmt.Sprintf("logger%d", i)))
			time.Sleep(time.Millisecond * 2)
		}
	}()
	
	// Goroutine to call Reset (but not too often to allow some processing)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			time.Sleep(time.Millisecond * 10)
			engine.Reset()
		}
	}()
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// The test passes if no race conditions are detected by the race detector
}
