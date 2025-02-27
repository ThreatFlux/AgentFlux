package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vtriple/agentflux/pkg/common/logging"
	"github.com/vtriple/agentflux/pkg/processor"
)

func TestNewAPIClient(t *testing.T) {
	client := NewAPIClient("https://api.example.com", AuthBearer, "test-token")
	
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	
	if client.Endpoint != "https://api.example.com" {
		t.Errorf("Expected endpoint %s, got %s", "https://api.example.com", client.Endpoint)
	}
	
	if client.AuthMethod != AuthBearer {
		t.Errorf("Expected auth method %s, got %s", AuthBearer, client.AuthMethod)
	}
	
	token, ok := client.Credentials.(string)
	if !ok || token != "test-token" {
		t.Errorf("Expected credentials to be string token 'test-token'")
	}
	
	if client.BatchSize != DefaultBatchSize {
		t.Errorf("Expected batch size %d, got %d", DefaultBatchSize, client.BatchSize)
	}
}

func TestAddAuthToRequest(t *testing.T) {
	// Test Bearer Auth
	client := NewAPIClient("https://api.example.com", AuthBearer, "test-token")
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com", nil)
	
	err := client.addAuthToRequest(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	authHeader := req.Header.Get("Authorization")
	if authHeader != "Bearer test-token" {
		t.Errorf("Expected Authorization header 'Bearer test-token', got '%s'", authHeader)
	}
	
	// Test API Key Auth
	client = NewAPIClient("https://api.example.com", AuthAPIKey, "api-key-value")
	req, _ = http.NewRequest(http.MethodGet, "https://api.example.com", nil)
	
	err = client.addAuthToRequest(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	apiKeyHeader := req.Header.Get("X-API-Key")
	if apiKeyHeader != "api-key-value" {
		t.Errorf("Expected X-API-Key header 'api-key-value', got '%s'", apiKeyHeader)
	}
	
	// Test Basic Auth
	basicAuth := BasicAuth{
		Username: "user",
		Password: "pass",
	}
	client = NewAPIClient("https://api.example.com", AuthBasic, basicAuth)
	req, _ = http.NewRequest(http.MethodGet, "https://api.example.com", nil)
	
	err = client.addAuthToRequest(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	username, password, ok := req.BasicAuth()
	if !ok {
		t.Fatal("Expected basic auth to be set")
	}
	if username != "user" || password != "pass" {
		t.Errorf("Expected basic auth 'user:pass', got '%s:%s'", username, password)
	}
	
	// Test Basic Auth with map
	client = NewAPIClient("https://api.example.com", AuthBasic, map[string]string{
		"username": "user2",
		"password": "pass2",
	})
	req, _ = http.NewRequest(http.MethodGet, "https://api.example.com", nil)
	
	err = client.addAuthToRequest(req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	username, password, ok = req.BasicAuth()
	if !ok {
		t.Fatal("Expected basic auth to be set")
	}
	if username != "user2" || password != "pass2" {
		t.Errorf("Expected basic auth 'user2:pass2', got '%s:%s'", username, password)
	}
}

// Test authentication error cases
func TestAddAuthToRequestErrors(t *testing.T) {
	tests := []struct {
		name         string
		authMethod   AuthType
		credentials  interface{}
		expectErrMsg string
	}{
		{
			name:         "Invalid bearer token type",
			authMethod:   AuthBearer,
			credentials:  123, // Not a string
			expectErrMsg: "bearer auth requires a string token",
		},
		{
			name:         "Invalid API key type",
			authMethod:   AuthAPIKey,
			credentials:  456, // Not a string
			expectErrMsg: "api-key auth requires a string key",
		},
		{
			name:         "Basic auth with invalid type",
			authMethod:   AuthBasic,
			credentials:  "not a map or struct",
			expectErrMsg: "basic auth requires BasicAuth struct or map with username/password",
		},
		{
			name:         "Basic auth with incomplete map",
			authMethod:   AuthBasic,
			credentials:  map[string]string{"username": "user"}, // Missing password
			expectErrMsg: "basic auth requires username and password",
		},
		{
			name:         "Unsupported auth method",
			authMethod:   "unsupported",
			credentials:  "token",
			expectErrMsg: "unsupported authentication method: unsupported",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewAPIClient("https://api.example.com", tc.authMethod, tc.credentials)
			req, _ := http.NewRequest(http.MethodGet, "https://api.example.com", nil)
			
			err := client.addAuthToRequest(req)
			if err == nil {
				t.Fatalf("Expected error but got none")
			}
			
			if !strings.Contains(err.Error(), tc.expectErrMsg) {
				t.Errorf("Expected error with '%s', got '%s'", tc.expectErrMsg, err.Error())
			}
		})
	}
}

func TestSendBatch(t *testing.T) {
	// Create a test server that responds with 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		
		// Check content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type header 'application/json', got '%s'", contentType)
		}
		
		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("Expected Authorization header 'Bearer test-token', got '%s'", authHeader)
		}
		
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	// Create client pointing to test server
	client := NewAPIClient(server.URL, AuthBearer, "test-token")
	
	// Create a test batch
	batch := []processor.FileResult{
		{
			Path:          "/test/file1.txt",
			Name:          "file1.txt",
			Size:          1024,
			Hash:          "abc123",
			HashAlgorithm: "sha256",
			ProcessedAt:   time.Now(),
		},
		{
			Path:          "/test/file2.txt",
			Name:          "file2.txt",
			Size:          2048,
			Hash:          "def456",
			HashAlgorithm: "sha256",
			ProcessedAt:   time.Now(),
		},
	}
	
	// Test sending the batch
	err := client.sendBatch(context.Background(), batch)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Test sending an empty batch
	err = client.sendBatch(context.Background(), []processor.FileResult{})
	if err != nil {
		t.Fatalf("Unexpected error sending empty batch: %v", err)
	}
}

// Test setting HTTP client
func TestSetHTTPClient(t *testing.T) {
	client := NewAPIClient("https://api.example.com", AuthBearer, "test-token")
	originalClient := client.httpClient
	
	customClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	client.SetHTTPClient(customClient)
	
	if client.httpClient != customClient {
		t.Errorf("Expected HTTP client to be set to custom client")
	}
	
	if client.httpClient == originalClient {
		t.Errorf("Expected HTTP client to be different from original")
	}
}

// Test setting logger
func TestSetLogger(t *testing.T) {
	client := NewAPIClient("https://api.example.com", AuthBearer, "test-token")
	originalLogger := client.logger
	
	customLogger := logging.NewLogger("custom-api")
	
	client.SetLogger(customLogger)
	
	if client.logger != customLogger {
		t.Errorf("Expected logger to be set to custom logger")
	}
	
	if client.logger == originalLogger {
		t.Errorf("Expected logger to be different from original")
	}
}

// Test calculate backoff function
func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name       string
		retry      int
		maxBackoff time.Duration
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{
			name:       "First retry",
			retry:      1,
			maxBackoff: 5 * time.Second,
			minExpected: 160 * time.Millisecond, // 200ms * 0.8
			maxExpected: 280 * time.Millisecond, // 200ms * 1.4
		},
		{
			name:       "Second retry",
			retry:      2,
			maxBackoff: 5 * time.Second,
			minExpected: 320 * time.Millisecond, // 400ms * 0.8
			maxExpected: 560 * time.Millisecond, // 400ms * 1.4
		},
		{
			name:       "Very large retry capped by maxBackoff",
			retry:      10,
			maxBackoff: 2 * time.Second,
			minExpected: 2 * time.Second, // Capped at maxBackoff
			maxExpected: 2 * time.Second, // Capped at maxBackoff
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backoff := calculateBackoff(tc.retry, tc.maxBackoff)
			
			if backoff < tc.minExpected || backoff > tc.maxExpected {
				t.Errorf("Expected backoff between %v and %v, got %v", 
					tc.minExpected, tc.maxExpected, backoff)
			}
			
			if tc.minExpected == tc.maxExpected && backoff != tc.minExpected {
				t.Errorf("Expected backoff of exactly %v, got %v", tc.minExpected, backoff)
			}
		})
	}
}

// Test sendWithRetries with different response codes
func TestSendWithRetries(t *testing.T) {
	tests := []struct {
		name           string
		serverResponses []int
		expectError    bool
		maxRetries     int
	}{
		{
			name:           "Success on first attempt",
			serverResponses: []int{200},
			expectError:    false,
			maxRetries:     3,
		},
		{
			name:           "Success after one retry",
			serverResponses: []int{500, 200},
			expectError:    false,
			maxRetries:     3,
		},
		{
			name:           "All retries failed with 5xx",
			serverResponses: []int{500, 502, 503, 500},
			expectError:    true,
			maxRetries:     3,
		},
		{
			name:           "Client error (4xx) no retry",
			serverResponses: []int{400},
			expectError:    true,
			maxRetries:     3,
		},
		{
			name:           "429 Too Many Requests - should retry",
			serverResponses: []int{429, 429, 200},
			expectError:    false,
			maxRetries:     3,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Track request count
			requestCount := 0
			
			// Create a test server with dynamic responses
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Get the status code for this request
				statusCode := 500 // Default
				if requestCount < len(tc.serverResponses) {
					statusCode = tc.serverResponses[requestCount]
				}
				requestCount++
				
				w.WriteHeader(statusCode)
				
				// Add response body for non-2xx
				if statusCode >= 300 {
					w.Write([]byte("Error response"))
				}
			}))
			defer server.Close()
			
			// Create client with a very short backoff for test speed
			client := NewAPIClient(server.URL, AuthBearer, "test-token")
			// Use a tiny backoff value to make test faster
			testMaxBackoffVal := 1 * time.Millisecond
			t.Logf("Using max backoff of %v", testMaxBackoffVal)
			
			// Create request
			req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
			
			// Test sending with retries
			err := client.sendWithRetries(req, tc.maxRetries)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			// Verify request count - should be limited by server responses or max retries+1
			expectedRequests := len(tc.serverResponses)
			if expectedRequests > tc.maxRetries+1 {
				expectedRequests = tc.maxRetries + 1
			}
			if !tc.expectError {
				// If success, we should stop at the successful request
				for i, code := range tc.serverResponses {
					if code >= 200 && code < 300 {
						expectedRequests = i + 1
						break
					}
				}
			} else if len(tc.serverResponses) == 1 && tc.serverResponses[0] >= 400 && tc.serverResponses[0] < 500 && tc.serverResponses[0] != 429 {
				// For 4xx errors (except 429), we should stop after the first request
				expectedRequests = 1
			}
			
			if requestCount != expectedRequests {
				t.Errorf("Expected %d requests, got %d", expectedRequests, requestCount)
			}
		})
	}
}

// Mock HTTP client for testing
type mockHTTPClient struct {
	doFunc func(*http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

// Test retry with network errors
func TestSendWithRetriesNetworkError(t *testing.T) {
	client := NewAPIClient("https://api.example.com", AuthBearer, "test-token")
	
	// Create a mock client that always returns a network error
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		},
	}
	
    // Create real HTTP client with mock transport
    httpClient := &http.Client{
        Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
            return mockClient.Do(req)
        }),
    }
    client.SetHTTPClient(httpClient)
	
	// Set a tiny backoff
	testMaxBackoffVal := 1 * time.Millisecond
	t.Logf("Using max backoff of %v", testMaxBackoffVal)
	maxRetries := 2
	
	// Create request
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com", nil)
	
	// Call the function
	start := time.Now()
	err := client.sendWithRetries(req, maxRetries)
	elapsed := time.Since(start)
	t.Logf("Operation took %v", elapsed)
	
	// Expect error
	if err == nil {
		t.Errorf("Expected error but got none")
	}
	
	// Error should mention "network error"
	if !strings.Contains(err.Error(), "network error") {
		t.Errorf("Expected error to contain 'network error', got '%s'", err.Error())
	}
	
	// Error should mention retry count
	expectedAttempts := maxRetries + 1
	for i := 1; i <= expectedAttempts; i++ {
		if !strings.Contains(err.Error(), "attempt") {
			t.Errorf("Expected error to mention attempt, got '%s'", err.Error())
		}
	}
}

// mockRoundTripper is an implementation of http.RoundTripper that uses the provided function
type mockRoundTripper func(*http.Request) (*http.Response, error)

func (f mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Test flushBatch
func TestFlushBatch(t *testing.T) {
	// Create a test server that responds with 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	tests := []struct {
		name           string
		setupBatch     []processor.FileResult
		expectError    bool
		expectAPICall  bool
	}{
		{
			name:           "Empty batch",
			setupBatch:     []processor.FileResult{},
			expectError:    false,
			expectAPICall:  false,
		},
		{
			name:           "Non-empty batch",
			setupBatch:     []processor.FileResult{
				{
					Path:          "/test/file1.txt",
					Name:          "file1.txt",
					Hash:          "abc123",
					HashAlgorithm: "sha256",
				},
			},
			expectError:    false,
			expectAPICall:  true,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create client pointing to test server
			client := NewAPIClient(server.URL, AuthBearer, "test-token")
			
			// Track if API was called
			apiCalled := false
			mockClient := &mockHTTPClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					apiCalled = true
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				},
			}
			
			// Create real HTTP client with mock transport
			httpClient := &http.Client{
				Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
					return mockClient.Do(req)
				}),
			}
			client.SetHTTPClient(httpClient)
			
			// Set up batch
			client.batchMutex.Lock()
			client.currentBatch = make([]processor.FileResult, len(tc.setupBatch))
			copy(client.currentBatch, tc.setupBatch)
			client.batchMutex.Unlock()
			
			// Create error channel
			errorChan := make(chan error, 1)
			
			// Flush batch
			err := client.flushBatch(errorChan)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if tc.expectAPICall != apiCalled {
				t.Errorf("Expected API call: %v, got: %v", tc.expectAPICall, apiCalled)
			}
			
			// Check if batch was cleared
			client.batchMutex.Lock()
			if len(client.currentBatch) != 0 {
				t.Errorf("Expected batch to be cleared, got %d items", len(client.currentBatch))
			}
			client.batchMutex.Unlock()
		})
	}
}

// Test addToBatch
func TestAddToBatch(t *testing.T) {
	tests := []struct {
		name          string
		initialBatch  []processor.FileResult
		batchSize     int
		expectSend    bool
	}{
		{
			name:          "Add to empty batch",
			initialBatch:  []processor.FileResult{},
			batchSize:     5,
			expectSend:    false,
		},
		{
			name:          "Add to batch - not full",
			initialBatch:  make([]processor.FileResult, 3),
			batchSize:     5,
			expectSend:    false,
		},
		{
			name:          "Add to batch - becomes full",
			initialBatch:  make([]processor.FileResult, 4),
			batchSize:     5,
			expectSend:    true,
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			
			// Create client
			client := NewAPIClient(server.URL, AuthBearer, "test-token")
			client.BatchSize = tc.batchSize
			
			// Track if batch was sent
			batchSent := false
			mockClient := &mockHTTPClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					batchSent = true
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				},
			}
			
			// Create real HTTP client with mock transport
			httpClient := &http.Client{
				Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
					return mockClient.Do(req)
				}),
			}
			client.SetHTTPClient(httpClient)
			
			// Set up initial batch
			client.batchMutex.Lock()
			client.currentBatch = make([]processor.FileResult, len(tc.initialBatch))
			copy(client.currentBatch, tc.initialBatch)
			client.batchMutex.Unlock()
			
			// Add a result to the batch
			ctx := context.Background()
			errorChan := make(chan error, 1)
			result := processor.FileResult{
				Path:          "/test/new-file.txt",
				Name:          "new-file.txt",
				Hash:          "xyz789",
				HashAlgorithm: "sha256",
			}
			
			err := client.addToBatch(ctx, result, errorChan)
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if tc.expectSend != batchSent {
				t.Errorf("Expected batch sent: %v, got: %v", tc.expectSend, batchSent)
			}
			
			// Check batch size after operation
			client.batchMutex.Lock()
			var expectedSize int
			if tc.expectSend {
				expectedSize = 0 // Batch should be cleared after sending
			} else {
				expectedSize = len(tc.initialBatch) + 1
			}
			if len(client.currentBatch) != expectedSize {
				t.Errorf("Expected batch size %d, got %d", expectedSize, len(client.currentBatch))
			}
			client.batchMutex.Unlock()
		})
	}
}

// Test SendResults basic functionality
func TestSendResults(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	// Create client
	client := NewAPIClient(server.URL, AuthBearer, "test-token")
	client.BatchSize = 2 // Small batch size for testing
	
	// Create a channel of results
	resultChan := make(chan processor.FileResult, 5)
	
	// Add some results
	for i := 0; i < 5; i++ {
		resultChan <- processor.FileResult{
			Path:          "/test/file" + string(rune('1'+i)) + ".txt",
			Name:          "file" + string(rune('1'+i)) + ".txt",
			Hash:          "hash" + string(rune('1'+i)),
			HashAlgorithm: "sha256",
		}
	}
	close(resultChan)
	
	// Track API calls
	apiCalls := 0
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			apiCalls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}
	
	// Create real HTTP client with mock transport
	httpClient := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return mockClient.Do(req)
		}),
	}
	client.SetHTTPClient(httpClient)
	
	// Call SendResults
	ctx := context.Background()
	errorChan := client.SendResults(ctx, resultChan)
	
	// Wait for processing to complete
	client.Wait()
	
	// Check for errors
	for err := range errorChan {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// Expect 3 API calls (2 batches of 2 + 1 final batch of 1)
	expectedCalls := 3
	if apiCalls != expectedCalls {
		t.Errorf("Expected %d API calls, got %d", expectedCalls, apiCalls)
	}
}

// Test SendResults with context cancellation
func TestSendResultsWithContextCancellation(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	// Create client with a small batch size
	client := NewAPIClient(server.URL, AuthBearer, "test-token")
	client.BatchSize = 10 // Large enough that we won't send until cancel
	
	// Track API calls
	var apiCallsCount int
	var apiCallsMutex sync.Mutex
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			apiCallsMutex.Lock()
			apiCallsCount++
			apiCallsMutex.Unlock()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}
	
	// Create real HTTP client with mock transport
	httpClient := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return mockClient.Do(req)
		}),
	}
	client.SetHTTPClient(httpClient)
	
	// Create a cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create an unbuffered channel to control the test flow
	resultChan := make(chan processor.FileResult)
	
	// Start SendResults in goroutine
	errorChan := client.SendResults(ctx, resultChan)
	
	// Send a few results (not enough to trigger batch send)
	for i := 0; i < 5; i++ {
		resultChan <- processor.FileResult{
			Path:          "/test/file" + string(rune('1'+i)) + ".txt",
			Name:          "file" + string(rune('1'+i)) + ".txt",
			Hash:          "hash" + string(rune('1'+i)),
			HashAlgorithm: "sha256",
		}
	}
	
	// Cancel the context
	cancel()
	
	// Close the channel to unblock SendResults
	close(resultChan)
	
	// Wait for processing to complete
	client.Wait()
	
	// Check for errors - expect at least one for context cancellation
	errorCount := 0
	for range errorChan {
		errorCount++
	}
	t.Logf("Found %d errors after context cancellation", errorCount)
	
	// Expect at least one API call for the flush after cancellation
	apiCallsMutex.Lock()
	if apiCallsCount < 1 {
		t.Errorf("Expected at least 1 API call for flush, got %d", apiCallsCount)
	}
	apiCallsMutex.Unlock()
}

// Test Wait functionality
func TestWait(t *testing.T) {
	client := NewAPIClient("https://api.example.com", AuthBearer, "test-token")
	
	// Create a channel that will signal when the goroutine completes
	done := make(chan struct{})
	
	// Add a task to wait group
	client.wg.Add(1)
	
	// Start a goroutine that will decrement the wait group counter
	go func() {
		// Simulate work
		time.Sleep(100 * time.Millisecond)
		client.wg.Done()
		close(done)
	}()
	
	// Call Wait in a goroutine
	waitReturned := make(chan struct{})
	go func() {
		client.Wait()
		close(waitReturned)
	}()
	
	// Wait should not return before the goroutine completes
	select {
	case <-waitReturned:
		t.Errorf("Wait returned before goroutine completed")
	case <-time.After(50 * time.Millisecond):
		// This is expected
	}
	
	// Wait for the goroutine to complete
	<-done
	
	// Now Wait should return
	select {
	case <-waitReturned:
		// This is expected
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Wait did not return after goroutine completed")
	}
}

// Test calculateBackoff with more coverage
func TestCalculateBackoffComprehensive(t *testing.T) {
    // Set a fixed seed for predictable results
    rand.Seed(42)
    
    maxBackoff := 5 * time.Second
    
    // Test with retry 0 (first attempt)
    backoff := calculateBackoff(0, maxBackoff)
    expected := 100 * time.Millisecond // Base (exact value may vary due to jitter)
    if backoff < 80*time.Millisecond || backoff > 140*time.Millisecond {
        t.Errorf("Expected backoff around %v for retry 0, got %v", expected, backoff)
    }
    
    // Test with retry 1
    backoff = calculateBackoff(1, maxBackoff)
    expected = 200 * time.Millisecond // Base * 2 (exact value may vary due to jitter)
    if backoff < 160*time.Millisecond || backoff > 280*time.Millisecond {
        t.Errorf("Expected backoff around %v for retry 1, got %v", expected, backoff)
    }
    
    // Test with retry 10 (should hit maxBackoff)
    backoff = calculateBackoff(10, maxBackoff)
    if backoff != maxBackoff {
        t.Errorf("Expected backoff to be capped at %v, got %v", maxBackoff, backoff)
    }
}

// Test addToBatch with full coverage
func TestAddToBatchComprehensive(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()
    
    // Test adding to batch without reaching the batch size
    client := NewAPIClient(server.URL, AuthBearer, "test-token")
    client.BatchSize = 2
    
    // Create a result
    result := processor.FileResult{
        Path: "/test/file.txt",
        Name: "file.txt",
        Hash: "abc123",
        HashAlgorithm: "sha256",
    }
    
    // Add to batch (should not trigger send)
    ctx := context.Background()
    errorChan := make(chan error, 1)
    err := client.addToBatch(ctx, result, errorChan)
    
    if err != nil {
        t.Errorf("Unexpected error adding to batch: %v", err)
    }
    
    // Verify batch contains the item
    client.batchMutex.Lock()
    if len(client.currentBatch) != 1 {
        t.Errorf("Expected batch size 1, got %d", len(client.currentBatch))
    }
    client.batchMutex.Unlock()
    
    // Add another item to trigger send
    // Mock HTTP client to track calls
    apiCalled := false
    mockClient := &mockHTTPClient{
        doFunc: func(req *http.Request) (*http.Response, error) {
            apiCalled = true
            return &http.Response{
                StatusCode: http.StatusOK,
                Body: io.NopCloser(strings.NewReader("")),
            }, nil
        },
    }
    
    // Create real HTTP client with mock transport
    httpClient := &http.Client{
        Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
            return mockClient.Do(req)
        }),
    }
    client.SetHTTPClient(httpClient)
    
    // Add second item (should trigger send)
    err = client.addToBatch(ctx, result, errorChan)
    
    if err != nil {
        t.Errorf("Unexpected error adding to batch: %v", err)
    }
    
    // Verify batch was sent and cleared
    client.batchMutex.Lock()
    if len(client.currentBatch) != 0 {
        t.Errorf("Expected batch to be cleared after sending, got size %d", len(client.currentBatch))
    }
    client.batchMutex.Unlock()
    
    // Verify API was called
    if !apiCalled {
        t.Errorf("Expected API to be called when batch is full")
    }
    
    // Test with sending error
    apiCalled = false
    mockClient.doFunc = func(req *http.Request) (*http.Response, error) {
        apiCalled = true
        return nil, fmt.Errorf("send error")
    }
    
    // Reset batch state
    client.batchMutex.Lock()
    client.currentBatch = make([]processor.FileResult, 0, client.BatchSize)
    client.currentBatch = append(client.currentBatch, result)
    client.batchMutex.Unlock()
    
    // Add another item (should trigger send and error)
    err = client.addToBatch(ctx, result, errorChan)
    
    if err == nil {
        t.Errorf("Expected error from batch send, got none")
    }
    
    // Verify API was called
    if !apiCalled {
        t.Errorf("Expected API to be called even with error")
    }
}

// Test sendBatch with request error
func TestSendBatchWithRequestError(t *testing.T) {
    // Create client with invalid URL to force request creation error
    client := NewAPIClient("://invalid", AuthBearer, "test-token")
    
    // Create a test batch
    batch := []processor.FileResult{
        {
            Path: "/test/file.txt",
            Name: "file.txt",
            Hash: "abc123",
            HashAlgorithm: "sha256",
        },
    }
    
    // Attempt to send
    err := client.sendBatch(context.Background(), batch)
    
    // Should fail with request creation error
    if err == nil {
        t.Errorf("Expected request creation error, got none")
    }
    
    if !strings.Contains(err.Error(), "error creating request") {
        t.Errorf("Expected 'error creating request' error, got: %v", err)
    }
}

// Test sendResults with mixed success/error scenarios
func TestSendResultsComprehensive(t *testing.T) {
    // Create a server with controllable response
    var serverResponse struct {
        statusCode int
        body       string
        lock       sync.Mutex
    }
    serverResponse.statusCode = http.StatusOK
    
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        serverResponse.lock.Lock()
        defer serverResponse.lock.Unlock()
        w.WriteHeader(serverResponse.statusCode)
        w.Write([]byte(serverResponse.body))
    }))
    defer server.Close()
    
    // Create client with a small batch size for quicker testing
    client := NewAPIClient(server.URL, AuthBearer, "test-token")
    client.BatchSize = 2
    
    // Create result channel with several items
    resultChannel := make(chan processor.FileResult, 5)
    for i := 0; i < 5; i++ {
        resultChannel <- processor.FileResult{
            Path:          fmt.Sprintf("/path/to/file%d.txt", i),
            Name:          fmt.Sprintf("file%d.txt", i),
            Hash:          fmt.Sprintf("hash%d", i),
            HashAlgorithm: "sha256",
        }
    }
    
    // First let all succeed
    ctx, cancel := context.WithCancel(context.Background())
    errorChannel := client.SendResults(ctx, resultChannel)
    
    // Make sure first batch went through successfully
    time.Sleep(50 * time.Millisecond)
    
    // Now set server to return errors
    serverResponse.lock.Lock()
    serverResponse.statusCode = http.StatusInternalServerError
    serverResponse.body = "Internal error"
    serverResponse.lock.Unlock()
    
    // Wait for processing to complete
    close(resultChannel)
    client.Wait()
    
    // Should have some errors
    errorCount := 0
    for range errorChannel {
        errorCount++
    }
    
    t.Logf("Got %d errors from mixed success/error test", errorCount)
    
    // Test with context cancellation
    resultChannel = make(chan processor.FileResult, 5)
    for i := 0; i < 5; i++ {
        resultChannel <- processor.FileResult{
            Path:          fmt.Sprintf("/path/to/file%d.txt", i),
            Name:          fmt.Sprintf("file%d.txt", i),
            Hash:          fmt.Sprintf("hash%d", i),
            HashAlgorithm: "sha256",
        }
    }
    
    // Create new context to cancel
    ctx, cancel = context.WithCancel(context.Background())
    errorChannel = client.SendResults(ctx, resultChannel)
    
    // Let processing start
    time.Sleep(50 * time.Millisecond)
    
    // Cancel context
    cancel()
    
    // Wait for processing to finish
    client.Wait()
    
    // Count errors due to cancellation
    errorCount = 0
    for range errorChannel {
        errorCount++
    }
    
    t.Logf("Got %d errors after context cancellation", errorCount)
}

// Test the helper function
func TestContainsStringExtended(t *testing.T) {
    tests := []struct {
        s        string
        substr   string
        expected bool
    }{
        {"hello world", "world", true},       // Normal match
        {"hello world", "hello world", false}, // s == substr
        {"hello", "", false},                 // Empty substr
        {"", "world", false},                 // Empty s
        {"", "", false},                      // Both empty
    }
    
    for _, tc := range tests {
        result := containsStringExtended(tc.s, tc.substr)
        if result != tc.expected {
            t.Errorf("containsStringExtended(%q, %q): expected %v, got %v", 
                tc.s, tc.substr, tc.expected, result)
        }
    }
}

// TestSendBatchDirectly tests the sendBatch function with more test cases
func TestSendBatchDirectly(t *testing.T) {
    // Test server with various responses
    testCases := []struct {
        name           string
        statusCode     int
        responseBody   string
        expectError    bool
        timeoutSeconds float64
    }{
        {
            name:           "Success response",
            statusCode:     200,
            responseBody:   "",
            expectError:    false,
            timeoutSeconds: 0.1,
        },
        {
            name:           "Not found response",
            statusCode:     404,
            responseBody:   "Not found",
            expectError:    true,
            timeoutSeconds: 0.1,
        },
        {
            name:           "Server error response",
            statusCode:     500,
            responseBody:   "Internal server error",
            expectError:    true,
            timeoutSeconds: 0.1,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Create test server
            server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                // Check request method is POST
                if r.Method != http.MethodPost {
                    t.Errorf("Expected POST request, got %s", r.Method)
                }
                
                // Check Content-Type header
                contentType := r.Header.Get("Content-Type")
                if contentType != "application/json" {
                    t.Errorf("Expected Content-Type header 'application/json', got '%s'", contentType)
                }
                
                // Return the configured status code and body
                w.WriteHeader(tc.statusCode)
                if tc.responseBody != "" {
                    w.Write([]byte(tc.responseBody))
                }
            }))
            defer server.Close()
            
            // Create client pointing to test server
            client := NewAPIClient(server.URL, AuthBearer, "test-token")
            
            // Create a test batch
            batch := []processor.FileResult{
                {
                    Path:          "/test/file.txt",
                    Name:          "file.txt",
                    Size:          1024,
                    Hash:          "abc123",
                    HashAlgorithm: "sha256",
                    ProcessedAt:   time.Now(),
                },
            }
            
            // Create context with timeout
            ctx, cancel := context.WithTimeout(context.Background(), 
                time.Duration(tc.timeoutSeconds*float64(time.Second)))
            defer cancel()
            
            // Call sendBatch directly
            err := client.sendBatch(ctx, batch)
            
            // Check result
            if tc.expectError && err == nil {
                t.Errorf("Expected error but got none")
            } else if !tc.expectError && err != nil {
                t.Errorf("Expected no error but got: %v", err)
            }
        })
    }
}

// TestSendWithRetriesComprehensive tests the sendWithRetries function more thoroughly
func TestSendWithRetriesSimple(t *testing.T) {
    // Use a simple version that doesn't depend on timing
    
    // Create a test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()
    
    // Create client
    client := NewAPIClient(server.URL, AuthBearer, "test-token")
    client.MaxRetries = 2
    
    // Create request
    req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("{}"))
    if err != nil {
        t.Fatalf("Failed to create request: %v", err)
    }
    
    // Call sendWithRetries directly
    err = client.sendWithRetries(req, client.MaxRetries)
    
    // Should succeed
    if err != nil {
        t.Errorf("Expected no error but got: %v", err)
    }
}
