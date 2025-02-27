// Package scanner provides file system scanning functionality.
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/vtriple/agentflux/pkg/common/logging"
)

// FileScanner scans file systems and returns file paths.
type FileScanner struct {
	// RootPaths is a list of paths to scan.
	RootPaths []string
	// ExcludePaths is a list of glob patterns to exclude.
	ExcludePaths []string
	// MaxDepth is the maximum directory depth to scan (-1 for unlimited).
	MaxDepth int
	// MaxFileSize is the maximum file size to include (in bytes).
	MaxFileSize int64
	// SkipSymlinks determines whether to skip symbolic links.
	SkipSymlinks bool
	// SkipHiddenFiles determines whether to skip hidden files.
	SkipHiddenFiles bool
	
	ctx    context.Context
	wg     sync.WaitGroup
	logger *logging.Logger
}

// NewFileScanner creates a new FileScanner with the specified context and root paths.
func NewFileScanner(ctx context.Context, paths []string) *FileScanner {
	return &FileScanner{
		RootPaths:       paths,
		MaxDepth:        -1,
		MaxFileSize:     -1,
		SkipSymlinks:    true,
		SkipHiddenFiles: true,
		ctx:             ctx,
		logger:          logging.NewLogger("scanner"),
	}
}

// Scan starts scanning the file system and returns channels for file paths and errors.
func (s *FileScanner) Scan() (<-chan string, <-chan error) {
	fileChannel := make(chan string, 1000)
	errorChannel := make(chan error, 100)
	
	// Start a goroutine for each root path
	s.wg.Add(len(s.RootPaths))
	for _, path := range s.RootPaths {
		rootPath := path // Capture variable for goroutine
		go func() {
			defer s.wg.Done()
			s.scanPath(rootPath, 0, fileChannel, errorChannel)
		}()
	}
	
	// Start a goroutine to close channels when done
	go func() {
		s.wg.Wait()
		close(fileChannel)
		close(errorChannel)
	}()
	
	return fileChannel, errorChannel
}

// isHiddenFile checks if a file or directory is hidden.
func isHiddenFile(filename string) bool {
	// In Unix-like systems, hidden files start with a dot
	return len(filename) > 0 && filename[0] == '.'
}

// scanPath recursively scans a directory and sends file paths to the channel.
func (s *FileScanner) scanPath(path string, depth int, fileChannel chan<- string, errorChannel chan<- error) {
	// Check if context is canceled
	select {
	case <-s.ctx.Done():
		return
	default:
		// Continue scanning
	}
	
	// Check depth limit
	if s.MaxDepth >= 0 && depth > s.MaxDepth {
		return
	}
	
	// Get file info for the path
	info, err := os.Lstat(path)
	if err != nil {
		select {
		case errorChannel <- fmt.Errorf("error accessing path %s: %w", path, err):
		default:
			// Channel might be full, log error
			s.logger.Error("Error channel full, could not send error: %v", err)
		}
		return
	}
	
	// Skip hidden files/directories if configured to do so
	filename := filepath.Base(path)
	if s.SkipHiddenFiles && isHiddenFile(filename) {
		return
	}
	
	// Handle symbolic links
	if info.Mode()&os.ModeSymlink != 0 {
		if s.SkipSymlinks {
			return
		}
		
		// Resolve symlink
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			select {
			case errorChannel <- fmt.Errorf("error resolving symlink %s: %w", path, err):
			default:
				s.logger.Error("Error channel full, could not send error: %v", err)
			}
			return
		}
		
		// Get info for the resolved path
		info, err = os.Stat(realPath)
		if err != nil {
			select {
			case errorChannel <- fmt.Errorf("error accessing resolved path %s: %w", realPath, err):
			default:
				s.logger.Error("Error channel full, could not send error: %v", err)
			}
			return
		}
		
		// Update path to the resolved path
		path = realPath
	}
	
	// Check if path should be excluded
	if s.shouldExclude(path) {
		return
	}
	
	// Handle directories
	if info.IsDir() {
		// Read directory entries
		entries, err := os.ReadDir(path)
		if err != nil {
			select {
			case errorChannel <- fmt.Errorf("error reading directory %s: %w", path, err):
			default:
				s.logger.Error("Error channel full, could not send error: %v", err)
			}
			return
		}
		
		// Recurse into subdirectories and process files
		for _, entry := range entries {
			// Skip hidden entries if configured
			if s.SkipHiddenFiles && isHiddenFile(entry.Name()) {
				continue
			}
			
			entryPath := filepath.Join(path, entry.Name())
			
			if entryInfo, err := entry.Info(); err == nil {
				if entryInfo.IsDir() {
					s.scanPath(entryPath, depth+1, fileChannel, errorChannel)
				} else {
					s.processFile(entryPath, entryInfo, fileChannel, errorChannel)
				}
			} else {
				select {
				case errorChannel <- fmt.Errorf("error getting info for %s: %w", entryPath, err):
				default:
					s.logger.Error("Error channel full, could not send error: %v", err)
				}
			}
		}
	} else {
		// Handle regular files
		s.processFile(path, info, fileChannel, errorChannel)
	}
}

// processFile processes a file and sends its path to the channel if it meets the criteria.
func (s *FileScanner) processFile(path string, info os.FileInfo, fileChannel chan<- string, errorChannel chan<- error) {
	// Check if context is canceled
	select {
	case <-s.ctx.Done():
		return
	default:
		// Continue processing
	}
	
	// Skip irregular files (devices, pipes, etc.)
	if !info.Mode().IsRegular() {
		return
	}
	
	// Check file size
	if s.MaxFileSize > 0 && info.Size() > s.MaxFileSize {
		return
	}
	
	// Check if file should be excluded
	if s.shouldExclude(path) {
		return
	}
	
	// Send file path to channel
	select {
	case fileChannel <- path:
		// Successfully sent
	case <-s.ctx.Done():
		// Context was canceled
		return
	}
}

// shouldExclude checks if a path should be excluded based on the exclude patterns.
func (s *FileScanner) shouldExclude(path string) bool {
	for _, pattern := range s.ExcludePaths {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
		
		// Also try matching against the full path
		matched, err = filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// SetContext updates the scanner's context.
func (s *FileScanner) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// SetLogger sets a custom logger for the scanner.
func (s *FileScanner) SetLogger(logger *logging.Logger) {
	s.logger = logger
}
