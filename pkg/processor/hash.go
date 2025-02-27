// Package processor provides functionality for processing files.
package processor

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode"

	"github.com/vtriple/agentflux/pkg/common/logging"
)

// FileResult contains information about a processed file.
type FileResult struct {
	// Path is the full path to the file.
	Path string `json:"path"`
	// Name is the filename without directory.
	Name string `json:"name"`
	// Size is the size of the file in bytes.
	Size int64 `json:"size"`
	// ModTime is the last modification time of the file.
	ModTime time.Time `json:"modTime"`
	// Hash is the computed hash of the file.
	Hash string `json:"hash"`
	// HashAlgorithm is the algorithm used to compute the hash.
	HashAlgorithm string `json:"hashAlgorithm"`
	// MimeType is the MIME type of the file, if detectable.
	MimeType string `json:"mimeType,omitempty"`
	// Strings is a list of extracted strings from the file.
	Strings []string `json:"strings,omitempty"`
	// Error is a description of any error that occurred during processing.
	Error string `json:"error,omitempty"`
	// IsExecutable indicates if the file has executable permissions.
	IsExecutable bool `json:"isExecutable,omitempty"`
	// ProcessedAt is when the file was processed.
	ProcessedAt time.Time `json:"processedAt"`
}

// HashProcessor computes hashes and extracts information from files.
type HashProcessor struct {
	// HashAlgorithm is the algorithm to use for hashing (md5, sha1, sha256, sha512).
	HashAlgorithm string
	// WorkerCount is the number of worker goroutines to use.
	WorkerCount int
	// ExtractStrings indicates whether to extract strings from files.
	ExtractStrings bool
	// StringMinLength is the minimum length for extracted strings.
	StringMinLength int
	// SkipLargeFiles indicates whether to skip files larger than MaxFileSize.
	SkipLargeFiles bool
	// MaxFileSize is the maximum file size to process.
	MaxFileSize int64
	
	wg     sync.WaitGroup
	logger *logging.Logger
}

// NewHashProcessor creates a new HashProcessor with the specified algorithm.
func NewHashProcessor(algorithm string, workers int) *HashProcessor {
	return &HashProcessor{
		HashAlgorithm:   algorithm,
		WorkerCount:     workers,
		ExtractStrings:  false,
		StringMinLength: 4,
		SkipLargeFiles:  true,
		MaxFileSize:     100 * 1024 * 1024, // 100MB default
		logger:          logging.NewLogger("processor"),
	}
}

// Process processes files from the input channel and returns a channel of results.
func (h *HashProcessor) Process(fileChannel <-chan string) <-chan FileResult {
	resultChannel := make(chan FileResult, h.WorkerCount*2)
	
	// Start worker goroutines
	h.wg.Add(h.WorkerCount)
	for i := 0; i < h.WorkerCount; i++ {
		workerID := i
		go func() {
			defer h.wg.Done()
			h.worker(workerID, fileChannel, resultChannel)
		}()
	}
	
	// Start closer goroutine
	go func() {
		h.wg.Wait()
		close(resultChannel)
	}()
	
	return resultChannel
}

// worker processes files from the input channel and sends results to the output channel.
func (h *HashProcessor) worker(id int, fileChannel <-chan string, resultChannel chan<- FileResult) {
	h.logger.Debug("Worker %d started", id)
	
	for filePath := range fileChannel {
		result := h.processFile(filePath)
		resultChannel <- result
	}
	
	h.logger.Debug("Worker %d finished", id)
}

// processFile processes a single file and returns a FileResult.
func (h *HashProcessor) processFile(filePath string) FileResult {
	result := FileResult{
		Path:          filePath,
		Name:          filepath.Base(filePath),
		HashAlgorithm: h.HashAlgorithm,
		ProcessedAt:   time.Now(),
	}
	
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("stat error: %v", err)
		return result
	}
	
	result.Size = fileInfo.Size()
	result.ModTime = fileInfo.ModTime()
	result.IsExecutable = fileInfo.Mode()&0111 != 0
	
	// Check if file is too large
	if h.SkipLargeFiles && result.Size > h.MaxFileSize {
		result.Error = fmt.Sprintf("file too large (%d bytes)", result.Size)
		return result
	}
	
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("open error: %v", err)
		return result
	}
	defer file.Close()
	
	// Calculate the hash
	hashValue, err := h.calculateHash(file)
	if err != nil {
		result.Error = fmt.Sprintf("hash error: %v", err)
		return result
	}
	result.Hash = hashValue
	
	// Extract strings if requested
	if h.ExtractStrings {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			result.Error = fmt.Sprintf("seek error: %v", err)
			return result
		}
		
		strings, err := h.extractStrings(file)
		if err != nil {
			result.Error = fmt.Sprintf("string extraction error: %v", err)
			return result
		}
		result.Strings = strings
	}
	
	return result
}

// calculateHash calculates the hash of a file using the specified algorithm.
func (h *HashProcessor) calculateHash(r io.Reader) (string, error) {
	var hasher hash.Hash
	
	switch h.HashAlgorithm {
	case "md5":
		hasher = md5.New()
	case "sha1":
		hasher = sha1.New()
	case "sha256":
		hasher = sha256.New()
	case "sha512":
		hasher = sha512.New()
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", h.HashAlgorithm)
	}
	
	// Use a buffer for more efficient I/O
	buf := make([]byte, 1024*1024) // 1MB buffer
	if _, err := io.CopyBuffer(hasher, r, buf); err != nil {
		return "", err
	}
	
	// Get the hash as a hexadecimal string
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// extractStrings extracts printable strings from a file.
func (h *HashProcessor) extractStrings(r io.Reader) ([]string, error) {
	result := make([]string, 0, 100)
	seenStrings := make(map[string]bool)
	
	scanner := bufio.NewScanner(r)
	scanner.Split(h.scanStrings)
	
	// Scan the file for strings
	for scanner.Scan() {
		str := scanner.Text()
		if len(str) >= h.StringMinLength && !seenStrings[str] {
			result = append(result, str)
			seenStrings[str] = true
			
			// Limit the number of strings to avoid excessive memory usage
			if len(result) >= 10000 {
				break
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	
	return result, nil
}

// scanStrings is a custom split function for bufio.Scanner that extracts strings.
func (h *HashProcessor) scanStrings(data []byte, atEOF bool) (advance int, token []byte, err error) {
	start := 0
	
	// Skip non-printable characters
	for start < len(data) {
		r := rune(data[start])
		if r > 127 || !unicode.IsPrint(r) {
			start++
			continue
		}
		break
	}
	
	// Find the end of the string
	end := start
	for end < len(data) {
		r := rune(data[end])
		if r > 127 || !unicode.IsPrint(r) {
			break
		}
		end++
	}
	
	// If we're at EOF and haven't found a complete string
	if atEOF && start >= len(data) {
		return len(data), nil, nil
	}
	
	// If we have a string
	if end > start {
		return end, data[start:end], nil
	}
	
	// Request more data
	return start, nil, nil
}

// SetLogger sets a custom logger for the processor.
func (h *HashProcessor) SetLogger(logger *logging.Logger) {
	h.logger = logger
}
