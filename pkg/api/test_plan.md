# API Package Test Coverage Improvement Plan

## Current Coverage: 27.3%

## Functions That Need Test Coverage

1. **SendResults(ctx, resultChannel)**
   - Test normal operation with a channel of results
   - Test context cancellation
   - Test when resultChannel closes prematurely
   - Test error handling

2. **addToBatch(ctx, result, errorChannel)**
   - Test adding a single result
   - Test when batch becomes full and triggers sending
   - Test error handling during batch sending
   - Test context cancellation

3. **flushBatch(errorChannel)**
   - Test flushing an empty batch
   - Test flushing a non-empty batch
   - Test error handling during sending

4. **sendWithRetries(req, maxRetries)**
   - Test successful request on first try
   - Test successful request after retries
   - Test various HTTP error codes (4xx, 5xx)
   - Test retry backoff logic
   - Test when all retries fail

5. **calculateBackoff(retry, maxBackoff)**
   - Test with different retry counts
   - Test with different maxBackoff values
   - Verify exponential growth
   - Verify capping at maxBackoff

6. **Wait()**
   - Test that it waits for operations to complete

7. **SetHTTPClient(client)**
   - Test that it replaces the default client

8. **SetLogger(logger)**
   - Test that it replaces the default logger

## Authentication Error Cases

9. **addAuthToRequest(req)**
   - Test error cases:
     - Invalid bearer token (not string)
     - Invalid API key (not string)
     - Invalid basic auth (wrong type)
     - Missing username/password for basic auth
     - Unsupported auth method

## Test Approach

1. Create mock HTTP server for testing API calls
2. Create mock context for testing cancellation
3. Set up test fixtures with various result sets
4. Create helper functions to validate correct behavior

## Implementation Plan

1. Create tests for basic functions (SetHTTPClient, SetLogger, Wait)
2. Implement tests for authentication error cases
3. Create tests for retry and backoff logic
4. Implement complex tests for SendResults full flow
5. Add tests for batch processing logic
