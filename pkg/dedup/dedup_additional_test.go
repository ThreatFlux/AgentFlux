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

// TestSetLogger_Direct tests the SetLogger method directly
func TestSetLogger_Direct(t *testing.T) {
	// Create a new deduplication engine
	engine := NewDeduplicationEngine(HashDedup)

	// Create a custom logger
	logger := logging.NewLogger("test-dedup")

	// Set the logger
	engine.SetLogger(logger)

	// Verify the logger was set correctly
	if engine.logger != logger {
		t.Errorf("Expected logger to be set to custom logger")
	}
}

// TestDeduplicationEngine_AdditionalCancellationTest tests the engine's behavior when the context is cancelled in a different way
func TestDeduplicationEngine_AdditionalCancellationTest(t *testing.T) {
	// Create engine
	engine := NewDeduplicationEngine(HashDedup)

	// Create a cancelable context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Set up channels
	inputChan := make(chan processor.FileResult)
	outputChan := engine.Deduplicate(ctx, inputChan)

	// Create a WaitGroup to wait for the test to complete
	var wg sync.WaitGroup
	wg.Add(1)

	// Track how many results are received
	receivedCount := 0

	// Start a goroutine to read results
	go func() {
		defer wg.Done()
		for range outputChan {
			receivedCount++
		}
	}()

	// Wait for context to timeout
	<-ctx.Done()

	// Send some results (these should not be processed due to timeout)
	for i := 0; i < 5; i++ {
		select {
		case inputChan <- processor.FileResult{
			Path:          "/path/to/file" + string(rune('1'+i)) + ".txt",
			Name:          "file" + string(rune('1'+i)) + ".txt",
			Hash:          "hash" + string(rune('1'+i)),
			HashAlgorithm: "sha256",
			Size:          int64(100 + i),
		}:
		default:
			// Channel might be closed already
		}
	}

	// Close the input channel
	close(inputChan)

	// Wait for processing to complete
	wg.Wait()

	// The test passes if it doesn't hang
}

// TestDeduplicationEngine_WithInvalidDeduplicationType tests behavior with an invalid deduplication type
func TestDeduplicationEngine_WithInvalidDeduplicationType(t *testing.T) {
	// Create engine with invalid type
	engine := NewDeduplicationEngine("invalid_type")

	// Create a result
	result := processor.FileResult{
		Path:          "/path/to/file.txt",
		Name:          "file.txt",
		HashAlgorithm: "sha256",
		Hash:          "abc123",
		Size:          100,
	}

	// Get key - should default to hash deduplication
	key := engine.getDeduplicationKey(result)
	expectedKey := "sha256:abc123"

	if key != expectedKey {
		t.Errorf("Expected key %s for invalid deduplication type, got %s", expectedKey, key)
	}

	// Process some results to ensure the engine still works
	ctx := context.Background()
	inputChan := make(chan processor.FileResult, 3)

	// Add some results with the same hash
	inputChan <- result
	inputChan <- processor.FileResult{
		Path:          "/path/to/file2.txt",
		Name:          "file2.txt",
		HashAlgorithm: "sha256",
		Hash:          "abc123", // Same hash
		Size:          100,
	}
	close(inputChan)

	outputChan := engine.Deduplicate(ctx, inputChan)

	// Count results
	resultCount := 0
	for range outputChan {
		resultCount++
	}

	// Should only get one result due to deduplication
	if resultCount != 1 {
		t.Errorf("Expected 1 result for invalid deduplication type, got %d", resultCount)
	}

	// Check stats
	total, unique := engine.GetStats()
	if total != 2 {
		t.Errorf("Expected 2 total files, got %d", total)
	}
	if unique != 1 {
		t.Errorf("Expected 1 unique file, got %d", unique)
	}
}

// TestDeduplicationEngine_DoneChannelSync tests the synchronization of the done channel
func TestDeduplicationEngine_DoneChannelSync(t *testing.T) {
	engine := NewDeduplicationEngine(HashDedup)

	// Create channels
	ctx := context.Background()
	inputChan := make(chan processor.FileResult)
	outputChan := engine.Deduplicate(ctx, inputChan)

	// Close the input channel immediately
	close(inputChan)

	// Wait for the output channel to close
	for range outputChan {
		// Consume all results
	}

	// The done channel should be closed
	select {
	case <-engine.done:
		// This is the expected path
	default:
		t.Error("Done channel not closed after operation completed")
	}

	// Reset the engine and check that the done channel is recreated
	engine.Reset()

	// The done channel should be open again
	select {
	case <-engine.done:
		t.Error("Done channel closed after reset")
	default:
		// This is the expected path
	}
}

// TestDeduplicationEngine_PathDedup tests path-based deduplication
func TestDeduplicationEngine_PathDedup(t *testing.T) {
	// Create engine with path deduplication
	engine := NewDeduplicationEngine(PathDedup)

	// Create a context
	ctx := context.Background()

	// Create channels
	inputChan := make(chan processor.FileResult, 10)

	// Add results with same paths but different hashes
	for i := 0; i < 3; i++ {
		// Same path, different hashes
		inputChan <- processor.FileResult{
			Path:          "/path/to/file.txt",
			Name:          "file.txt",
			HashAlgorithm: "sha256",
			Hash:          fmt.Sprintf("hash%d", i),
			Size:          100,
		}
	}

	// Add results with different paths
	for i := 0; i < 3; i++ {
		// Different paths
		inputChan <- processor.FileResult{
			Path:          fmt.Sprintf("/path/to/file%d.txt", i),
			Name:          fmt.Sprintf("file%d.txt", i),
			HashAlgorithm: "sha256",
			Hash:          "samehash", // Same hash
			Size:          100,
		}
	}

	close(inputChan)

	// Deduplicate
	outputChan := engine.Deduplicate(ctx, inputChan)

	// Count results
	var results []processor.FileResult
	for result := range outputChan {
		results = append(results, result)
	}

	// Should have 4 results:
	// 1 for the duplicate path and 3 for the unique paths
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	// Verify stats
	total, unique := engine.GetStats()
	if total != 6 {
		t.Errorf("Expected 6 total files, got %d", total)
	}
	if unique != 4 {
		t.Errorf("Expected 4 unique files, got %d", unique)
	}
}

// TestDeduplicationEngine_NameDedup tests name-based deduplication
func TestDeduplicationEngine_NameDedup(t *testing.T) {
	// Create engine with name deduplication
	engine := NewDeduplicationEngine(NameDedup)

	// Create a context
	ctx := context.Background()

	// Create channels
	inputChan := make(chan processor.FileResult, 10)

	// Add results with same names but different paths
	for i := 0; i < 3; i++ {
		// Same name, different paths
		inputChan <- processor.FileResult{
			Path:          fmt.Sprintf("/path%d/to/file.txt", i),
			Name:          "file.txt",
			HashAlgorithm: "sha256",
			Hash:          fmt.Sprintf("hash%d", i),
			Size:          100,
		}
	}

	// Add results with different names
	for i := 0; i < 3; i++ {
		// Different names
		inputChan <- processor.FileResult{
			Path:          "/path/to/file",
			Name:          fmt.Sprintf("file%d.txt", i),
			HashAlgorithm: "sha256",
			Hash:          "samehash", // Same hash
			Size:          100,
		}
	}

	close(inputChan)

	// Deduplicate
	outputChan := engine.Deduplicate(ctx, inputChan)

	// Count results
	var results []processor.FileResult
	for result := range outputChan {
		results = append(results, result)
	}

	// Should have 4 results:
	// 1 for the duplicate name and 3 for the unique names
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	// Verify stats
	total, unique := engine.GetStats()
	if total != 6 {
		t.Errorf("Expected 6 total files, got %d", total)
	}
	if unique != 4 {
		t.Errorf("Expected 4 unique files, got %d", unique)
	}
}

// TestDeduplicationEngine_CustomDedup tests custom size-based deduplication
func TestDeduplicationEngine_CustomDedup(t *testing.T) {
	// Create a custom deduplication type based on size
	sizeDedup := DeduplicationType("size")

	// Create engine with custom deduplication
	engine := NewDeduplicationEngine(sizeDedup)

	// Create a context
	ctx := context.Background()

	// Create channels
	inputChan := make(chan processor.FileResult, 10)

	// Add results with same size but different hashes
	for i := 0; i < 3; i++ {
		// Same size, different hashes
		inputChan <- processor.FileResult{
			Path:          fmt.Sprintf("/path/to/file%d.txt", i),
			Name:          fmt.Sprintf("file%d.txt", i),
			HashAlgorithm: "sha256",
			Hash:          fmt.Sprintf("hash%d", i),
			Size:          100, // Same size
		}
	}

	// Add results with different sizes
	for i := 0; i < 3; i++ {
		// Different sizes
		inputChan <- processor.FileResult{
			Path:          fmt.Sprintf("/path/to/other%d.txt", i),
			Name:          fmt.Sprintf("other%d.txt", i),
			HashAlgorithm: "sha256",
			Hash:          "samehash",
			Size:          int64(200 + i),
		}
	}

	close(inputChan)

	// Deduplicate
	outputChan := engine.Deduplicate(ctx, inputChan)

	// Count results
	var results []processor.FileResult
	for result := range outputChan {
		results = append(results, result)
	}

	// Check the results - since engine.go defaults to hash deduplication for
	// unknown types, we should see deduplication by hash, not by size
	_, unique := engine.GetStats()
	if unique != 4 {
		t.Errorf("Expected 4 unique files, got %d", unique)
	}
}

// TestDeduplicationEngine_ConcurrentReset tests resetting the engine while it's in use
func TestDeduplicationEngine_ConcurrentReset(t *testing.T) {
	engine := NewDeduplicationEngine(HashDedup)

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create channels with a large buffer to avoid blocking
	inputChan := make(chan processor.FileResult, 1000)

	// Start deduplication
	outputChan := engine.Deduplicate(ctx, inputChan)

	// Create a waitgroup to wait for processing to complete
	var wg sync.WaitGroup
	wg.Add(1)

	// Start a goroutine to consume results
	go func() {
		defer wg.Done()
		for range outputChan {
			// Just consume results
		}
	}()

	// Send some results
	for i := 0; i < 10; i++ {
		inputChan <- processor.FileResult{
			Path:          fmt.Sprintf("/path/to/file%d.txt", i),
			Name:          fmt.Sprintf("file%d.txt", i),
			HashAlgorithm: "sha256",
			Hash:          fmt.Sprintf("hash%d", i),
			Size:          int64(100 + i),
		}
	}

	// Reset the engine while it's processing
	engine.Reset()

	// Send more results
	for i := 10; i < 20; i++ {
		inputChan <- processor.FileResult{
			Path:          fmt.Sprintf("/path/to/file%d.txt", i),
			Name:          fmt.Sprintf("file%d.txt", i),
			HashAlgorithm: "sha256",
			Hash:          fmt.Sprintf("hash%d", i),
			Size:          int64(100 + i),
		}
	}

	// Close the input channel
	close(inputChan)

	// Wait for processing to complete
	wg.Wait()

	// The test passes if it completes without deadlock
	// We can't reliably verify the exact counts due to concurrent operations
}

// TestDeduplicationEngine_AdditionalConcurrencyTest tests concurrent operations on the engine
func TestDeduplicationEngine_AdditionalConcurrencyTest(t *testing.T) {
	engine := NewDeduplicationEngine(HashDedup)

	// Create a context
	ctx := context.Background()

	// Number of concurrent operations
	concurrency := 3
	resultsPerGoroutine := 20

	// Create a channel for results
	inputChan := make(chan processor.FileResult, concurrency*resultsPerGoroutine)

	// Start deduplication
	outputChan := engine.Deduplicate(ctx, inputChan)

	// Create a waitgroup for producers
	var producerWg sync.WaitGroup
	producerWg.Add(concurrency)

	// Start multiple goroutines to send results concurrently
	for g := 0; g < concurrency; g++ {
		go func(goroutineID int) {
			defer producerWg.Done()

			baseOffset := goroutineID * resultsPerGoroutine

			for i := 0; i < resultsPerGoroutine; i++ {
				// Use a combination of unique and duplicate values
				resultID := baseOffset + i
				hash := fmt.Sprintf("hash%d", resultID%10) // Create some duplicates

				result := processor.FileResult{
					Path:          fmt.Sprintf("/path/to/file%d.txt", resultID),
					Name:          fmt.Sprintf("file%d.txt", resultID%5), // More duplicates
					HashAlgorithm: "sha256",
					Hash:          hash,
					Size:          int64(100 + (resultID % 3)), // Even more duplicates
				}

				// Send the result
				inputChan <- result

				// Occasionally call GetStats to check for race conditions
				if i%5 == 0 {
					engine.GetStats()
				}
			}
		}(g)
	}

	// Close the input channel when all producers are done
	go func() {
		producerWg.Wait()
		close(inputChan)
	}()

	// Create a waitgroup for the consumer
	var consumerWg sync.WaitGroup
	consumerWg.Add(1)

	// Consume results
	go func() {
		defer consumerWg.Done()
		resultCount := 0

		for range outputChan {
			resultCount++

			// Occasionally call Reset to check for race conditions
			if resultCount%20 == 0 {
				engine.Reset()
			}
		}
	}()

	// Wait for all processing to complete
	consumerWg.Wait()

	// The test passes if there are no race condition failures
}
