// Package integration provides integration tests for the AgentFlux application.
// These tests require a running application and mock API server.
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vtriple/agentflux/pkg/processor"
)

// TestEndToEndFlow tests the entire flow from scanning files to API submission
func TestEndToEndFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary test directory
	testDir, err := os.MkdirTemp("", "agentflux-integration")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create test files
	setupTestFiles(t, testDir)

	// Start mock API server
	receivedResults := make([]processor.FileResult, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type header 'application/json', got '%s'", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header 'Bearer test-token', got '%s'", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Read and parse the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var results []processor.FileResult
		if err := json.Unmarshal(body, &results); err != nil {
			t.Errorf("Failed to parse request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Add the results to our collection
		receivedResults = append(receivedResults, results...)

		// Return success response
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Build the application for testing
	binPath, err := BuildForTest()
	if err != nil {
		t.Fatalf("Failed to build application: %v", err)
	}

	// Run the application
	cmd := exec.Command(
		binPath,
		"--paths="+testDir,
		"--algorithm=sha256",
		"--workers=2",
		"--api="+server.URL,
		"--token=test-token",
		"--strings",
		"--string-min=4",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run application: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify the results
	if len(receivedResults) == 0 {
		t.Fatal("No results were received by the API server")
	}

	// Verify file count - there's only 5 files but we may get 4 due to deduplication
	minExpectedFiles := 4 // At least the unique files should be there
	if len(receivedResults) < minExpectedFiles {
		t.Errorf("Expected at least %d files, got %d", minExpectedFiles, len(receivedResults))
	}

	// Verify file content
	for _, result := range receivedResults {
		// Verify the result has all required fields
		if result.Path == "" {
			t.Errorf("File result missing path")
		}
		if result.Name == "" {
			t.Errorf("File result missing name")
		}
		if result.Hash == "" {
			t.Errorf("File result missing hash")
		}
		if result.HashAlgorithm != "sha256" {
			t.Errorf("Expected hash algorithm 'sha256', got '%s'", result.HashAlgorithm)
		}
	}
}

// setupTestFiles creates test files in the specified directory
func setupTestFiles(t *testing.T, dir string) {
	// Create various test files
	testFiles := []struct {
		name    string
		content string
	}{
		{
			name:    "file1.txt",
			content: "This is a test file 1",
		},
		{
			name:    "file2.txt",
			content: "This is a test file 2",
		},
		{
			name:    "duplicate1.txt",
			content: "This is a duplicate file",
		},
		{
			name:    "duplicate2.txt",
			content: "This is a duplicate file",
		},
		{
			name:    "subdir/nested.txt",
			content: "This is a nested file",
		},
	}

	for _, tf := range testFiles {
		path := filepath.Join(dir, tf.name)

		// Create subdirectories if needed
		if dir := filepath.Dir(path); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}

		if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}
}
