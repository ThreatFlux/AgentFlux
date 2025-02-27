package dedup

import (
	"context"
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