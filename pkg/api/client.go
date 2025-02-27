// Package api provides functionality to send file processing results to an API endpoint.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/vtriple/agentflux/pkg/common/logging"
	"github.com/vtriple/agentflux/pkg/processor"
)

// AuthType represents the authentication method used for API requests.
type AuthType string

const (
	// AuthBearer uses a bearer token for authentication.
	AuthBearer AuthType = "bearer"
	// AuthBasic uses basic authentication.
	AuthBasic AuthType = "basic"
	// AuthAPIKey uses an API key for authentication.
	AuthAPIKey AuthType = "api-key"

	// DefaultBatchSize is the default number of results to send in a single batch.
	DefaultBatchSize = 100
	// DefaultMaxRetries is the default number of retry attempts for failed requests.
	DefaultMaxRetries = 3
	// DefaultMaxBackoff is the maximum backoff duration between retries.
	DefaultMaxBackoff = 5 * time.Second
	// DefaultErrorBufferSize is the default size of the error channel buffer.
	DefaultErrorBufferSize = 10
)

// BasicAuth contains username and password for basic authentication.
type BasicAuth struct {
	Username string
	Password string
}

// APIClient handles sending batches of file results to a remote API.
type APIClient struct {
	// Endpoint is the URL where file results are sent.
	Endpoint string
	// AuthMethod specifies the type of authentication to use.
	AuthMethod AuthType
	// Credentials stores authentication credentials according to AuthMethod.
	Credentials interface{}
	// BatchSize is the maximum number of results to send in a single request.
	BatchSize int
	// MaxRetries is the maximum number of retry attempts for a failed request.
	MaxRetries int
	// UserAgent is the user agent string sent with requests.
	UserAgent string

	httpClient   *http.Client
	currentBatch []processor.FileResult
	batchMutex   sync.Mutex
	wg           sync.WaitGroup
	logger       *logging.Logger
}

// NewAPIClient creates a new instance of APIClient.
func NewAPIClient(endpoint string, authMethod AuthType, credentials interface{}) *APIClient {
	return &APIClient{
		Endpoint:    endpoint,
		AuthMethod:  authMethod,
		Credentials: credentials,
		BatchSize:   DefaultBatchSize,
		MaxRetries:  DefaultMaxRetries,
		UserAgent:   "FileHashAgent/1.0",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          100,
				MaxConnsPerHost:       20,
				MaxIdleConnsPerHost:   10,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DisableCompression:    false,
				ForceAttemptHTTP2:     true,
			},
		},
		currentBatch: make([]processor.FileResult, 0, DefaultBatchSize),
		logger:       logging.NewLogger("api"),
	}
}

// SendResults processes a channel of file results and sends them to the API server.
// It returns a channel that will receive any errors encountered during processing.
func (a *APIClient) SendResults(ctx context.Context, resultChannel <-chan processor.FileResult) <-chan error {
	errorChannel := make(chan error, DefaultErrorBufferSize)
	
	// Start the processing goroutine
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer close(errorChannel)
		
		a.logger.Info("Starting to process results")
		
		// Process results until channel is closed
		for {
			select {
			case <-ctx.Done():
				a.logger.Info("Context cancelled, flushing remaining results")
				// Flush any remaining results
				if err := a.flushBatch(errorChannel); err != nil {
					select {
					case errorChannel <- fmt.Errorf("error flushing final batch: %w", err):
					default:
						// Channel might be full, log error
						a.logger.Error("Error channel full, error flushing batch: %v", err)
					}
				}
				return
				
			case result, ok := <-resultChannel:
				if !ok {
					// Channel closed, flush remaining batch
					a.logger.Info("Result channel closed, flushing remaining results")
					if err := a.flushBatch(errorChannel); err != nil {
						select {
						case errorChannel <- fmt.Errorf("error flushing final batch: %w", err):
						default:
							a.logger.Error("Error channel full, error flushing batch: %v", err)
						}
					}
					return
				}
				
				// Add to current batch
				if err := a.addToBatch(ctx, result, errorChannel); err != nil {
					select {
					case errorChannel <- fmt.Errorf("error adding to batch: %w", err):
					default:
						a.logger.Error("Error channel full, could not send error: %v", err)
					}
				}
			}
		}
	}()
	
	return errorChannel
}

// addToBatch adds a result to the current batch and sends the batch if it's full.
func (a *APIClient) addToBatch(ctx context.Context, result processor.FileResult, errorChannel chan<- error) error {
	a.batchMutex.Lock()
	defer a.batchMutex.Unlock()
	
	// Add to current batch
	a.currentBatch = append(a.currentBatch, result)
	
	// If batch is full, send it
	if len(a.currentBatch) >= a.BatchSize {
		// Create a copy of the current batch to send
		batch := make([]processor.FileResult, len(a.currentBatch))
		copy(batch, a.currentBatch)
		a.currentBatch = a.currentBatch[:0] // Clear the batch but preserve capacity
		
		// Release the lock before sending to avoid blocking other operations
		a.batchMutex.Unlock()
		
		if err := a.sendBatch(ctx, batch); err != nil {
			// Re-acquire the lock since we're using defer
			a.batchMutex.Lock()
			return fmt.Errorf("failed to send batch: %w", err)
		}
		
		// Re-acquire the lock since we're using defer
		a.batchMutex.Lock()
	}
	
	return nil
}

// flushBatch sends any remaining results in the current batch.
func (a *APIClient) flushBatch(errorChannel chan<- error) error {
	a.batchMutex.Lock()
	defer a.batchMutex.Unlock()
	
	if len(a.currentBatch) == 0 {
		return nil
	}
	
	// Create a copy of the current batch
	batch := make([]processor.FileResult, len(a.currentBatch))
	copy(batch, a.currentBatch)
	a.currentBatch = a.currentBatch[:0] // Clear the batch but preserve capacity
	
	// Use background context for flush operations if the main context is done
	return a.sendBatch(context.Background(), batch)
}

// sendBatch sends a batch of results to the API.
func (a *APIClient) sendBatch(ctx context.Context, batch []processor.FileResult) error {
	// Skip empty batches
	if len(batch) == 0 {
		return nil
	}
	
	// Marshal the batch to JSON
	jsonData, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("error marshaling batch: %w", err)
	}
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", a.UserAgent)
	
	// Add authentication
	if err := a.addAuthToRequest(req); err != nil {
		return fmt.Errorf("authentication error: %w", err)
	}
	
	// Send request with retries
	a.logger.Debug("Sending batch of %d items to API", len(batch))
	return a.sendWithRetries(req, a.MaxRetries)
}

// addAuthToRequest adds authentication headers to an HTTP request.
func (a *APIClient) addAuthToRequest(req *http.Request) error {
	switch a.AuthMethod {
	case AuthBearer:
		token, ok := a.Credentials.(string)
		if !ok {
			return fmt.Errorf("bearer auth requires a string token")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		
	case AuthBasic:
		switch creds := a.Credentials.(type) {
		case BasicAuth:
			req.SetBasicAuth(creds.Username, creds.Password)
		case map[string]string:
			username, hasUsername := creds["username"]
			password, hasPassword := creds["password"]
			if !hasUsername || !hasPassword {
				return fmt.Errorf("basic auth requires username and password")
			}
			req.SetBasicAuth(username, password)
		default:
			return fmt.Errorf("basic auth requires BasicAuth struct or map with username/password")
		}
		
	case AuthAPIKey:
		key, ok := a.Credentials.(string)
		if !ok {
			return fmt.Errorf("api-key auth requires a string key")
		}
		req.Header.Set("X-API-Key", key)
		
	default:
		return fmt.Errorf("unsupported authentication method: %s", a.AuthMethod)
	}
	
	return nil
}

// sendWithRetries sends a request with retries on failure.
func (a *APIClient) sendWithRetries(req *http.Request, maxRetries int) error {
	var lastErr error
	
	for retries := 0; retries <= maxRetries; retries++ {
		if retries > 0 {
			// Apply exponential backoff with jitter
			backoff := calculateBackoff(retries, DefaultMaxBackoff)
			a.logger.Debug("Retrying request after %v (attempt %d/%d)", backoff, retries, maxRetries)
			time.Sleep(backoff)
		}
		
		// Send request
		resp, err := a.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request error (attempt %d/%d): %w", retries+1, maxRetries+1, err)
			a.logger.Debug("HTTP request failed: %v", err)
			continue
		}
		
		// Check response status
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success, drain and close the body
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil
		}
		
		// Read error response
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		if err != nil {
			respBody = []byte("[error reading response body]")
		}
		
		lastErr = fmt.Errorf("API error (attempt %d/%d): status=%d, body=%s", 
			retries+1, maxRetries+1, resp.StatusCode, string(respBody))
		
		a.logger.Debug("API request failed: %v", lastErr)
		
		// Don't retry if client error (except 429 Too Many Requests)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			break
		}
	}
	
	return lastErr
}

// calculateBackoff calculates the backoff duration for retries with jitter.
func calculateBackoff(retry int, maxBackoff time.Duration) time.Duration {
	// Base exponential backoff: 2^retry * 100ms
	backoff := time.Duration(1<<uint(retry)) * 100 * time.Millisecond
	
	// Add jitter (±20%)
	jitter := time.Duration(float64(backoff) * (0.8 + 0.4*rand.Float64()))
	
	// Cap at maximum backoff
	if jitter > maxBackoff {
		jitter = maxBackoff
	}
	
	return jitter
}

// Wait waits for all pending operations to complete.
func (a *APIClient) Wait() {
	a.wg.Wait()
}

// SetHTTPClient allows setting a custom HTTP client.
func (a *APIClient) SetHTTPClient(client *http.Client) {
	a.httpClient = client
}

// SetLogger sets a custom logger for the API client.
func (a *APIClient) SetLogger(logger *logging.Logger) {
	a.logger = logger
}
