package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	
	"github.com/vtriple/agentflux/pkg/common/logging"
	"github.com/vtriple/agentflux/pkg/processor"
)

// TestSetHTTPClientAdditional tests additional scenarios for setting HTTP client
func TestSetHTTPClientAdditional(t *testing.T) {
	// Create a new API client
	client := NewAPIClient("https://example.com", AuthBearer, "test-token")
	
	// Create a custom HTTP client
	customClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	// Set the custom client
	client.SetHTTPClient(customClient)
	
	// Verify the client was set correctly
	if client.httpClient != customClient {
		t.Errorf("Expected HTTP client to be set to custom client")
	}
}

// TestSetLoggerAdditional tests additional scenarios for setting logger
func TestSetLoggerAdditional(t *testing.T) {
	// Create a new API client
	client := NewAPIClient("https://example.com", AuthBearer, "test-token")
	
	// Create a custom logger
	logger := logging.NewLogger("test-api")
	
	// Set the custom logger
	client.SetLogger(logger)
	
	// Verify the logger was set correctly
	if client.logger != logger {
		t.Errorf("Expected logger to be set to custom logger")
	}
}

// TestCalculateBackoffAdditional tests additional backoff scenarios
func TestCalculateBackoffAdditional(t *testing.T) {
	// Set seed for consistent test results
	maxBackoff := 5 * time.Second
	
	tests := []struct {
		name      string
		retry     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{
			name:      "First retry additional",
			retry:     1,
			minExpected: 160 * time.Millisecond, // 0.8 * 200ms
			maxExpected: 280 * time.Millisecond, // 1.4 * 200ms
		},
		{
			name:      "Second retry additional",
			retry:     2,
			minExpected: 320 * time.Millisecond, // 0.8 * 400ms
			maxExpected: 560 * time.Millisecond, // 1.4 * 400ms
		},
		{
			name:      "Very large retry additional",
			retry:     10,
			minExpected: time.Duration(float64(maxBackoff) * 0.8),
			maxExpected: maxBackoff,
		},
	}
	
	// Run tests multiple times to check range of jitter
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Using max backoff of %v", maxBackoff)
			for i := 0; i < 5; i++ {
				backoff := calculateBackoff(tc.retry, maxBackoff)
				
				// Check that backoff is within expected range
				if backoff < tc.minExpected || backoff > tc.maxExpected {
					t.Errorf("Expected backoff between %v and %v, got %v", 
						tc.minExpected, tc.maxExpected, backoff)
				}
			}
		})
	}
}

// TestSendWithRetriesNetworkErrorAdditional tests additional retry scenarios
func TestSendWithRetriesNetworkErrorAdditional(t *testing.T) {
	t.Logf("Using max backoff of %v", time.Millisecond) // Very small for testing
	
	// Create client with invalid URL to force network error
	client := NewAPIClient("invalid://url", AuthBearer, "test-token")
	client.MaxRetries = 3
	
	// Create a test request
	req, _ := http.NewRequestWithContext(
		context.Background(), 
		http.MethodGet, 
		"invalid://url", 
		nil,
	)
	
	// Measure time to test retries
	start := time.Now()
	
	// Send with retries
	err := client.sendWithRetries(req, client.MaxRetries)
	
	duration := time.Since(start)
	t.Logf("Operation took %v", duration)
	
	// Verify we got an error
	if err == nil {
		t.Errorf("Expected network error, got none")
	}
}

// TestFlushBatchAdditional tests the flushBatch method with additional scenarios
func TestFlushBatchAdditional(t *testing.T) {
	// Subtests
	tests := []struct {
		name        string
		batchSize   int
		expectError bool
	}{
		{
			name:        "Empty batch",
			batchSize:   0,
			expectError: false,
		},
		{
			name:        "Non-empty batch",
			batchSize:   3,
			expectError: false, // Should flush successfully during test
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test server that succeeds for this test
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Read request to verify batch was sent
				if tc.batchSize > 0 {
					var batch []processor.FileResult
					err := json.NewDecoder(r.Body).Decode(&batch)
					if err != nil {
						t.Errorf("Failed to decode request body: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					
					// Verify batch size
					if len(batch) != tc.batchSize {
						t.Errorf("Expected batch size %d, got %d", tc.batchSize, len(batch))
					}
				}
				
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			
			// Create client
			client := NewAPIClient(server.URL, AuthBearer, "test-token")
			
			// Create a mock error channel
			errorChannel := make(chan error, 10)
			
			// Add items to batch if needed
			if tc.batchSize > 0 {
				for i := 0; i < tc.batchSize; i++ {
					client.currentBatch = append(client.currentBatch, processor.FileResult{
						Path:          fmt.Sprintf("/path/to/file%d.txt", i),
						Name:          fmt.Sprintf("file%d.txt", i),
						HashAlgorithm: "sha256",
						Hash:          fmt.Sprintf("hash%d", i),
					})
				}
			}
			
			// Flush batch
			err := client.flushBatch(errorChannel)
			
			// Check result
			if tc.expectError && err == nil {
				t.Errorf("Expected error, got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			
			// Verify batch was cleared
			if len(client.currentBatch) != 0 {
				t.Errorf("Expected batch to be cleared, got %d items remaining", len(client.currentBatch))
			}
			
			// Check error channel
			close(errorChannel)
			for err := range errorChannel {
				t.Errorf("Unexpected error in channel: %v", err)
			}
		})
	}
}

// TestAddToBatchAdditional tests additional scenarios for adding items to batch
func TestAddToBatchAdditional(t *testing.T) {
	// Subtests
	tests := []struct {
		name           string
		initialBatch   int
		batchSize      int
		expectSend     bool
	}{
		{
			name:           "Add to empty batch",
			initialBatch:   0,
			batchSize:      5,
			expectSend:     false,
		},
		{
			name:           "Add to batch - not full",
			initialBatch:   2,
			batchSize:      5,
			expectSend:     false,
		},
		{
			name:           "Add to batch - becomes full",
			initialBatch:   4,
			batchSize:      5,
			expectSend:     true,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Track if batch was sent
			batchSent := false
			
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				batchSent = true
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			
			// Create client
			client := NewAPIClient(server.URL, AuthBearer, "test-token")
			client.BatchSize = tc.batchSize
			
			// Setup error channel
			errorChan := make(chan error, 1)
			defer close(errorChan)
			
			// Add initial items
			for i := 0; i < tc.initialBatch; i++ {
				client.currentBatch = append(client.currentBatch, processor.FileResult{
					Path: fmt.Sprintf("/path/to/file%d.txt", i),
					Name: fmt.Sprintf("file%d.txt", i),
				})
			}
			
			// Add one more item
			err := client.addToBatch(context.Background(), processor.FileResult{
				Path: "/path/to/new_file.txt",
				Name: "new_file.txt",
			}, errorChan)
			
			// Check results
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			
			// Check if batch was sent
			if tc.expectSend != batchSent {
				t.Errorf("Expected batch sent = %v, got %v", tc.expectSend, batchSent)
			}
			
			// Check batch size
			expectedBatchSize := tc.initialBatch + 1
			if tc.expectSend {
				expectedBatchSize = 0 // Batch cleared after sending
			}
			
			if len(client.currentBatch) != expectedBatchSize {
				t.Errorf("Expected batch size %d, got %d", expectedBatchSize, len(client.currentBatch))
			}
		})
	}
}

// TestSendResultsAdditional tests additional scenarios for SendResults
func TestSendResultsAdditional(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	// Create client
	client := NewAPIClient(server.URL, AuthBearer, "test-token")
	client.BatchSize = 3 // Small for testing
	
	// Create a channel with a few results
	resultChannel := make(chan processor.FileResult, 5)
	for i := 0; i < 5; i++ {
		resultChannel <- processor.FileResult{
			Path: fmt.Sprintf("/path/to/file%d.txt", i),
			Name: fmt.Sprintf("file%d.txt", i),
		}
	}
	close(resultChannel)
	
	// Send results
	errorChannel := client.SendResults(context.Background(), resultChannel)
	
	// Wait for completion
	client.Wait()
	
	// Check for errors
	errorCount := 0
	for range errorChannel {
		errorCount++
	}
	
	// Verify no errors
	if errorCount > 0 {
		t.Errorf("Expected no errors, got %d", errorCount)
	}
}

// TestSendResultsWithContextCancellationAdditional tests additional cancellation scenarios
func TestSendResultsWithContextCancellationAdditional(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep to make the requests take time
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	// Create client
	client := NewAPIClient(server.URL, AuthBearer, "test-token")
	client.BatchSize = 10 // Larger batch size to prevent batch sending
	
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create large unbuffered channel so it doesn't complete immediately
	resultChannel := make(chan processor.FileResult)
	
	// Start a goroutine to feed results slowly
	go func() {
		defer close(resultChannel)
		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				return
			case resultChannel <- processor.FileResult{
				Path: fmt.Sprintf("/path/to/file%d.txt", i),
				Name: fmt.Sprintf("file%d.txt", i),
			}:
				time.Sleep(5 * time.Millisecond) // Slow feed
			}
		}
	}()
	
	// Send results
	errorChannel := client.SendResults(ctx, resultChannel)
	
	// Cancel after short time
	time.Sleep(20 * time.Millisecond)
	cancel()
	
	// Wait for completion
	client.Wait()
	
	// Check for errors
	errorCount := 0
	for range errorChannel {
		errorCount++
	}
	
	// Log the error count - may vary depending on timing
	t.Logf("Found %d errors after context cancellation", errorCount)
}

// TestWaitAdditional tests additional Wait behavior
func TestWaitAdditional(t *testing.T) {
	// Create client
	client := NewAPIClient("https://example.com", AuthBearer, "test-token")
	
	// Increment wait group to simulate ongoing work
	client.wg.Add(1)
	
	// Start a goroutine that will decrement after a delay
	go func() {
		// Wait 100ms before completing
		time.Sleep(100 * time.Millisecond)
		client.wg.Done()
	}()
	
	// Measure time waiting
	start := time.Now()
	client.Wait()
	duration := time.Since(start)
	
	// Wait should have blocked until goroutine completed
	if duration < 100*time.Millisecond {
		t.Errorf("Wait should have blocked for at least 100ms, but took %v", duration)
	}
}

// TestAPIClient_AuthMethods tests all authentication methods
func TestAPIClient_AuthMethods(t *testing.T) {
	tests := []struct {
		name          string
		authMethod    AuthType
		credentials   interface{}
		expectedAuth  string
		expectedValue string
	}{
		{
			name:          "Bearer Token",
			authMethod:    AuthBearer,
			credentials:   "test-bearer-token",
			expectedAuth:  "Authorization",
			expectedValue: "Bearer test-bearer-token",
		},
		{
			name:          "API Key",
			authMethod:    AuthAPIKey,
			credentials:   "test-api-key",
			expectedAuth:  "X-API-Key",
			expectedValue: "test-api-key",
		},
		{
			name:          "Basic Auth",
			authMethod:    AuthBasic,
			credentials:   BasicAuth{Username: "testuser", Password: "testpass"},
			expectedAuth:  "Authorization",
			expectedValue: "Basic", // We'll just check prefix since encoding is handled by net/http
		},
		{
			name:          "Basic Auth from Map",
			authMethod:    AuthBasic,
			credentials:   map[string]string{"username": "testuser", "password": "testpass"},
			expectedAuth:  "Authorization",
			expectedValue: "Basic", // We'll just check prefix since encoding is handled by net/http
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var receivedHeaders http.Header

			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedHeaders = r.Header
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create client with test auth method
			client := NewAPIClient(server.URL, tc.authMethod, tc.credentials)

			// Create a simple request to test auth
			req, err := http.NewRequest("GET", server.URL, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Add authentication to request
			err = client.addAuthToRequest(req)
			if err != nil {
				t.Fatalf("Failed to add auth: %v", err)
			}

			// Send request
			_, err = client.httpClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			// Check authentication header was set correctly
			authHeader := receivedHeaders.Get(tc.expectedAuth)
			if authHeader == "" {
				t.Errorf("Expected %s header, got none", tc.expectedAuth)
			} else if !strings.HasPrefix(authHeader, tc.expectedValue) {
				t.Errorf("Expected %s to start with %s, got %s", tc.expectedAuth, tc.expectedValue, authHeader)
			}
		})
	}
}

// TestAPIClient_AuthMethodErrors tests error cases for authentication methods
func TestAPIClient_AuthMethodErrors(t *testing.T) {
	tests := []struct {
		name        string
		authMethod  AuthType
		credentials interface{}
		expectError bool
	}{
		{
			name:        "Bearer Token with Wrong Type",
			authMethod:  AuthBearer,
			credentials: 123, // Not a string
			expectError: true,
		},
		{
			name:        "API Key with Wrong Type",
			authMethod:  AuthAPIKey,
			credentials: []string{"not", "a", "string"},
			expectError: true,
		},
		{
			name:        "Basic Auth with Invalid Struct",
			authMethod:  AuthBasic,
			credentials: "not a struct or map",
			expectError: true,
		},
		{
			name:        "Basic Auth with Invalid Map",
			authMethod:  AuthBasic,
			credentials: map[string]string{"user": "wrong-key"}, // Missing username/password
			expectError: true,
		},
		{
			name:        "Invalid Auth Method",
			authMethod:  "invalid-method",
			credentials: "test-token",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create client with test auth method
			client := NewAPIClient("https://example.com", tc.authMethod, tc.credentials)

			// Create a simple request
			req, err := http.NewRequest("GET", "https://example.com", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Add authentication to request
			err = client.addAuthToRequest(req)

			// Check error
			if tc.expectError && err == nil {
				t.Errorf("Expected error, got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

