package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vtriple/agentflux/pkg/processor"
)

// TestAPIClient_CompleteWorkflow tests the entire client workflow
func TestAPIClient_CompleteWorkflow(t *testing.T) {
	// Create a test server with request tracking
	var requestsMutex sync.Mutex
	var receivedBatches [][]processor.FileResult
	var requestHeaders []http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authentication
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Read and parse request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var batch []processor.FileResult
		if err := json.Unmarshal(body, &batch); err != nil {
			t.Errorf("Failed to unmarshal request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Store batch and headers
		requestsMutex.Lock()
		receivedBatches = append(receivedBatches, batch)
		requestHeaders = append(requestHeaders, r.Header)
		requestsMutex.Unlock()

		// Return success
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client
	client := NewAPIClient(server.URL, AuthBearer, "test-token")
	client.BatchSize = 3 // Small batch size for testing

	// Create a channel of test results
	resultChannel := make(chan processor.FileResult, 10)
	for i := 0; i < 10; i++ {
		resultChannel <- processor.FileResult{
			Path:          fmt.Sprintf("/path/to/file%d.txt", i),
			Name:          fmt.Sprintf("file%d.txt", i),
			Size:          int64(100 * i),
			Hash:          fmt.Sprintf("hash%d", i),
			HashAlgorithm: "sha256",
			ProcessedAt:   time.Now(),
		}
	}
	close(resultChannel)

	// Start sending results
	ctx := context.Background()
	errorChannel := client.SendResults(ctx, resultChannel)

	// Wait for processing to complete
	client.Wait()

	// Check for errors
	for err := range errorChannel {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check results
	requestsMutex.Lock()
	defer requestsMutex.Unlock()

	// Should have 4 batches (3 batches of 3 files + 1 batch of 1 file)
	expectedBatches := 4
	if len(receivedBatches) != expectedBatches {
		t.Errorf("Expected %d batches, got %d", expectedBatches, len(receivedBatches))
	}

	// Check first three batches have 3 files each
	for i := 0; i < 3 && i < len(receivedBatches); i++ {
		if len(receivedBatches[i]) != 3 {
			t.Errorf("Batch %d: expected 3 files, got %d", i, len(receivedBatches[i]))
		}
	}

	// Check last batch has 1 file
	if len(receivedBatches) == 4 && len(receivedBatches[3]) != 1 {
		t.Errorf("Last batch: expected 1 file, got %d", len(receivedBatches[3]))
	}

	// Check all files were sent (10 in total)
	totalFiles := 0
	for _, batch := range receivedBatches {
		totalFiles += len(batch)
	}
	if totalFiles != 10 {
		t.Errorf("Expected 10 total files, got %d", totalFiles)
	}
}

// TestAPIClient_SendBatchServerErrors tests handling of different server errors
func TestAPIClient_SendBatchServerErrors(t *testing.T) {
	tests := []struct {
		name               string
		responseCode       int
		responseBody       string
		expectedErrorMatch string
		shouldRetry        bool
	}{
		{
			name:               "Bad Request",
			responseCode:       http.StatusBadRequest,
			responseBody:       "Invalid request format",
			expectedErrorMatch: "400",
			shouldRetry:        false, // Client errors don't retry
		},
		{
			name:               "Unauthorized",
			responseCode:       http.StatusUnauthorized,
			responseBody:       "Invalid credentials",
			expectedErrorMatch: "401",
			shouldRetry:        false,
		},
		{
			name:               "Not Found",
			responseCode:       http.StatusNotFound,
			responseBody:       "Endpoint not found",
			expectedErrorMatch: "404",
			shouldRetry:        false,
		},
		{
			name:               "Rate Limited",
			responseCode:       http.StatusTooManyRequests,
			responseBody:       "Rate limited",
			expectedErrorMatch: "429",
			shouldRetry:        true, // 429 should trigger retry
		},
		{
			name:               "Server Error",
			responseCode:       http.StatusInternalServerError,
			responseBody:       "Internal server error",
			expectedErrorMatch: "500",
			shouldRetry:        true, // Server errors should retry
		},
		{
			name:               "Service Unavailable",
			responseCode:       http.StatusServiceUnavailable,
			responseBody:       "Service unavailable",
			expectedErrorMatch: "503",
			shouldRetry:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Track number of requests
			requestCount := 0

			// Create a test server that responds with the specified status code
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				w.WriteHeader(tc.responseCode)
				w.Write([]byte(tc.responseBody))
			}))
			defer server.Close()

			// Create client with only 1 retry for faster tests
			client := NewAPIClient(server.URL, AuthBearer, "test-token")
			client.MaxRetries = 1

			// Create a small batch
			batch := []processor.FileResult{
				{
					Path:          "/path/to/file.txt",
					Name:          "file.txt",
					Hash:          "abcdef",
					HashAlgorithm: "sha256",
				},
			}

			// Use the exported method SendResults to test the internal sendBatch functionality
			resultChan := make(chan processor.FileResult, 1)
			resultChan <- batch[0]
			close(resultChan)
			
			// Get the error channel
			errChan := client.SendResults(context.Background(), resultChan)
			
			// Wait for completion
			client.Wait()
			
			// Check errors
			var err error
			for e := range errChan {
				err = e
				break
			}

			// Check if we got the expected error
			if err == nil && tc.expectedErrorMatch != "" {
				t.Errorf("Expected error for status %d, got none", tc.responseCode)
			}

			// Check error message if we got an error
			if err != nil && !strings.Contains(err.Error(), tc.expectedErrorMatch) {
				t.Errorf("Expected error to contain '%s', got: %v", 
					tc.expectedErrorMatch, err)
			}

			// Check retry behavior
			expectedRequests := 1
			if tc.shouldRetry {
				expectedRequests = 2 // Initial + 1 retry
			}

			if requestCount != expectedRequests {
				t.Errorf("Expected %d requests, got %d", expectedRequests, requestCount)
			}
		})
	}
}

// TestAPIClient_RetryBackoff tests the retry backoff mechanism
func TestAPIClient_RetryBackoff(t *testing.T) {
	// Create a test server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create client with multiple retries
	client := NewAPIClient(server.URL, AuthBearer, "test-token")
	client.MaxRetries = 3

	// Create a batch and send through the public API
	resultChan := make(chan processor.FileResult, 1)
	resultChan <- processor.FileResult{
		Path:          "/path/to/file.txt",
		Name:          "file.txt",
		Hash:          "abcdef",
		HashAlgorithm: "sha256",
	}
	close(resultChan)

	// Measure time for operation
	start := time.Now()
	errChan := client.SendResults(context.Background(), resultChan)
	client.Wait()
	duration := time.Since(start)

	// Verify we got an error
	var err error
	for e := range errChan {
		err = e
		break
	}

	// Verify error
	if err == nil {
		t.Errorf("Expected error, got none")
	}

	// Verify operation took time for retries (at least some backoff)
	// With 3 retries, even minimum backoff should take some time
	minExpectedTime := 100 * time.Millisecond
	if duration < minExpectedTime {
		t.Errorf("Expected backoff to delay operation by at least %v, got %v", 
			minExpectedTime, duration)
	}

	t.Logf("Operation with retries took %v", duration)
}

// TestAPIClient_SendBatchContextCancellation tests behavior when context is cancelled
func TestAPIClient_SendBatchContextCancellation(t *testing.T) {
	// Create a server that hangs for a while
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than context timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a context that will be cancelled soon
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Create client
	client := NewAPIClient(server.URL, AuthBearer, "test-token")

	// Create a result channel with one item
	resultChan := make(chan processor.FileResult, 1)
	resultChan <- processor.FileResult{
		Path:          "/path/to/file.txt",
		Name:          "file.txt",
		Hash:          "abcdef",
		HashAlgorithm: "sha256",
	}
	close(resultChan)

	// Send batch with the soon-to-be-cancelled context
	errChan := client.SendResults(ctx, resultChan)
	
	// Let Wait fail due to context cancellation
	client.Wait()
	
	// Check if we got an error
	var err error
	for e := range errChan {
		err = e
		break
	}

	// Verify error
	if err == nil {
		t.Errorf("Expected error from context cancellation, got none")
	}

	// Check if the error is related to context
	if err != nil && !strings.Contains(err.Error(), "context") && 
		!strings.Contains(err.Error(), "deadline") && 
		!strings.Contains(err.Error(), "canceled") {
		t.Errorf("Expected context-related error, got: %v", err)
	}
}

// TestAPIClient_SendBatchWithFailedMarshaling tests error propagation from JSON marshaling
func TestAPIClient_SendBatchWithFailedMarshaling(t *testing.T) {
	// This test is tricky because we can't easily create a JSON marshaling error
	// with the limited FileResult struct. Skip this test for now.
	t.Skip("Skipping marshal error test - would require more complex test setup")
}

// TestAPIClient_SendBatchWithRequestCreationError tests HTTP request creation failure
func TestAPIClient_SendBatchWithRequestCreationError(t *testing.T) {
	// Create client with invalid URL
	client := NewAPIClient("http://invalid-url-with-\x00-null-byte", AuthBearer, "test-token")

	// Create a result channel with one item
	resultChan := make(chan processor.FileResult, 1)
	resultChan <- processor.FileResult{
		Path:          "/path/to/file.txt",
		Name:          "file.txt",
		Hash:          "abcdef",
		HashAlgorithm: "sha256",
	}
	close(resultChan)

	// Send using the public API
	errChan := client.SendResults(context.Background(), resultChan)
	client.Wait()
	
	// Check if we got an error
	var err error
	for e := range errChan {
		err = e
		break
	}

	// Verify error
	if err == nil {
		t.Errorf("Expected error from request creation, got none")
	}

	// Error should mention request creation
	if err != nil && !strings.Contains(err.Error(), "request") {
		t.Errorf("Expected request creation error, got: %v", err)
	}
}

// TestAPIClient_AddToBatch_ErrorPropagation tests error propagation
func TestAPIClient_AddToBatch_ErrorPropagation(t *testing.T) {
	// We can't directly test the unexported addToBatch method
	// Instead, we'll test error propagation through the public SendResults API
	
	// Create a server that always succeeds but we'll never reach it
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with an invalid URL to generate errors
	client := NewAPIClient("invalid://url", AuthBearer, "test-token")
	client.BatchSize = 1 // Set to 1 so single add triggers send

	// Create a test result
	resultChan := make(chan processor.FileResult, 1)
	resultChan <- processor.FileResult{
		Path:          "/path/to/file.txt",
		Name:          "file.txt",
		Hash:          "abcdef",
		HashAlgorithm: "sha256",
	}
	close(resultChan)

	// Send results
	errChan := client.SendResults(context.Background(), resultChan)
	client.Wait()
	
	// Check for errors
	errCount := 0
	for range errChan {
		errCount++
	}

	// Verify we got at least one error
	if errCount == 0 {
		t.Errorf("Expected at least one error to be propagated, got none")
	}
}

// TestAPIClient_FlushBatch_EmptyBatch indirectly tests flushing an empty batch
func TestAPIClient_FlushBatch_EmptyBatch(t *testing.T) {
	// Create client
	client := NewAPIClient("https://example.com", AuthBearer, "test-token")

	// Create an empty channel and close it immediately to trigger a flush of empty batch
	resultChan := make(chan processor.FileResult)
	close(resultChan)

	// Send results should trigger a flush with empty batch
	errChan := client.SendResults(context.Background(), resultChan)
	client.Wait()
	
	// Check for errors - there should be none for empty batch
	errCount := 0
	for range errChan {
		errCount++
	}

	// Verify no errors
	if errCount > 0 {
		t.Errorf("Expected no errors for empty batch, got %d errors", errCount)
	}
}

// TestAPIClient_SendResults_ErrorChannelCapacity tests error channel capacity
func TestAPIClient_SendResults_ErrorChannelCapacity(t *testing.T) {
	// Create client with invalid URL to generate errors
	client := NewAPIClient("invalid://url", AuthBearer, "test-token")

	// Create a channel with many results to trigger errors
	resultChannel := make(chan processor.FileResult, DefaultErrorBufferSize+10)
	for i := 0; i < DefaultErrorBufferSize+5; i++ {
		resultChannel <- processor.FileResult{
			Path:          fmt.Sprintf("/path/to/error_file%d.txt", i),
			Name:          fmt.Sprintf("error_file%d.txt", i),
			Hash:          "abcdef",
			HashAlgorithm: "sha256",
		}
	}
	close(resultChannel)

	// Start sending results - this will generate errors since URL is invalid
	errorChannel := client.SendResults(context.Background(), resultChannel)

	// Wait for processing to complete
	client.Wait()

	// Count errors from channel
	errorCount := 0
	for range errorChannel {
		errorCount++
	}

	// We should get errors up to channel capacity
	if errorCount > DefaultErrorBufferSize {
		t.Errorf("Expected max %d errors due to channel capacity, got %d", 
			DefaultErrorBufferSize, errorCount)
	}

	// We should get at least some errors
	if errorCount == 0 {
		t.Errorf("Expected at least some errors, got none")
	}
}

// Helper function to check if a string contains a substring
// Function containsString is defined in helper_functions.go
