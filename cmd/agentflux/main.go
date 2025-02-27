// Package main provides the entry point for the agentflux application
// which scans file systems, computes file hashes, extracts strings, and
// sends this information to an API backend.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/vtriple/agentflux/pkg/api"
	"github.com/vtriple/agentflux/pkg/common/config"
	"github.com/vtriple/agentflux/pkg/common/logging"
	"github.com/vtriple/agentflux/pkg/dedup"
	"github.com/vtriple/agentflux/pkg/processor"
	"github.com/vtriple/agentflux/pkg/scanner"
)

// Version information
var (
	Version   = "1.0.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Set up a logger
	logger := logging.NewLogger("main")
	
	// Parse command line flags
	cfg, err := parseFlags()
	if err != nil {
		logger.Fatal("Error parsing flags: %v", err)
	}
	
	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		logger.Info("Received signal %v, initiating shutdown...", sig)
		cancel()
		
		// Force exit after a timeout if graceful shutdown is too slow
		time.AfterFunc(30*time.Second, func() {
			logger.Error("Graceful shutdown timed out, forcing exit")
			os.Exit(1)
		})
	}()
	
	// Run the main application
	if err := run(ctx, cfg, logger); err != nil {
		logger.Fatal("Application error: %v", err)
	}
}

// parseFlags parses command line flags and validates them
func parseFlags() (*config.Config, error) {
	cfg := &config.Config{}
	
	// Basic options
	flag.StringVar(&cfg.RootPaths, "paths", ".", "Comma-separated list of paths to scan")
	flag.StringVar(&cfg.ExcludePaths, "exclude", "", "Comma-separated list of glob patterns to exclude")
	flag.StringVar(&cfg.HashAlgorithm, "algorithm", "sha256", "Hash algorithm (md5, sha1, sha256, sha512)")
	flag.IntVar(&cfg.WorkerCount, "workers", runtime.NumCPU(), "Number of worker goroutines")
	flag.IntVar(&cfg.MaxDepth, "depth", -1, "Maximum directory depth (-1 for unlimited)")
	
	// API options
	flag.StringVar(&cfg.APIEndpoint, "api", "", "API endpoint URL")
	flag.StringVar(&cfg.APIToken, "token", "", "API authentication token")
	flag.StringVar(&cfg.APIAuthMethod, "auth-method", "bearer", "API auth method (bearer, basic, api-key)")
	flag.IntVar(&cfg.APIBatchSize, "batch", 100, "API batch size")
	
	// String extraction options
	flag.BoolVar(&cfg.ExtractStrings, "strings", false, "Extract strings from files")
	flag.IntVar(&cfg.StringMinLength, "string-min", 4, "Minimum string length to extract")
	
	// File processing options
	flag.Int64Var(&cfg.MaxFileSize, "max-size", 100*1024*1024, "Maximum file size to process in bytes")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&cfg.LogFile, "log-file", "", "Path to log file (empty for stderr)")
	
	// Misc options
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Show version information")
	
	flag.Parse()
	
	// Handle version flag
	if cfg.ShowVersion {
		fmt.Printf("AgentFlux v%s\n", Version)
		fmt.Printf("Build Date: %s\n", BuildDate)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}
	
	// Validate arguments
	if cfg.APIEndpoint == "" {
		return nil, fmt.Errorf("API endpoint is required")
	}
	
	// Validate hash algorithm
	validAlgs := map[string]bool{"md5": true, "sha1": true, "sha256": true, "sha512": true}
	if !validAlgs[strings.ToLower(cfg.HashAlgorithm)] {
		return nil, fmt.Errorf("unsupported hash algorithm: %s", cfg.HashAlgorithm)
	}
	
	// Parse root paths
	cfg.ParsedRootPaths = splitCSV(cfg.RootPaths)
	if len(cfg.ParsedRootPaths) == 0 {
		return nil, fmt.Errorf("at least one path must be specified")
	}
	
	// Parse exclude paths
	cfg.ParsedExcludePaths = splitCSV(cfg.ExcludePaths)
	
	return cfg, nil
}

// run executes the main application logic with the parsed configuration
func run(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	// Configure logging
	logging.SetGlobalLevel(cfg.LogLevel)
	if cfg.LogFile != "" {
		if err := logging.SetLogFile(cfg.LogFile); err != nil {
			return fmt.Errorf("failed to set log file: %w", err)
		}
	}
	
	// Create file scanner
	logger.Info("Initializing file scanner with %d paths", len(cfg.ParsedRootPaths))
	fileScanner := scanner.NewFileScanner(ctx, cfg.ParsedRootPaths)
	fileScanner.ExcludePaths = cfg.ParsedExcludePaths
	fileScanner.MaxDepth = cfg.MaxDepth
	fileScanner.MaxFileSize = cfg.MaxFileSize
	fileScanner.SetLogger(logging.NewLogger("scanner"))
	
	// Create hash processor
	logger.Info("Initializing hash processor with algorithm %s and %d workers", 
		cfg.HashAlgorithm, cfg.WorkerCount)
	hashProcessor := processor.NewHashProcessor(cfg.HashAlgorithm, cfg.WorkerCount)
	hashProcessor.ExtractStrings = cfg.ExtractStrings
	hashProcessor.StringMinLength = cfg.StringMinLength
	hashProcessor.SetLogger(logging.NewLogger("processor"))
	
	// Create deduplication engine
	logger.Info("Initializing deduplication engine")
	dedupEngine := dedup.NewDeduplicationEngine(dedup.HashDedup)
	dedupEngine.SetLogger(logging.NewLogger("dedup"))
	
	// Create API client
	logger.Info("Initializing API client with endpoint %s", cfg.APIEndpoint)
	apiClient := api.NewAPIClient(cfg.APIEndpoint, api.AuthType(cfg.APIAuthMethod), cfg.APIToken)
	apiClient.BatchSize = cfg.APIBatchSize
	apiClient.SetLogger(logging.NewLogger("api"))
	
	// Start the scanning process
	logger.Info("Starting file scan...")
	startTime := time.Now()
	
	// Set up the processing pipeline
	fileChannel, scanErrors := fileScanner.Scan()
	resultChannel := hashProcessor.Process(fileChannel)
	uniqueChannel := dedupEngine.Deduplicate(ctx, resultChannel)
	apiErrors := apiClient.SendResults(ctx, uniqueChannel)
	
	// Monitor for scan errors
	scanErrorCount := 0
	go func() {
		for err := range scanErrors {
			scanErrorCount++
			logger.Error("Scan error: %v", err)
		}
	}()
	
	// Monitor for API errors
	apiErrorCount := 0
	go func() {
		for err := range apiErrors {
			apiErrorCount++
			logger.Error("API error: %v", err)
		}
	}()
	
	// Wait for API client to finish
	apiClient.Wait()
	
	// Print summary
	elapsed := time.Since(startTime)
	totalFiles, uniqueFiles := dedupEngine.GetStats()
	
	logger.Info("Scan completed in %s", elapsed)
	logger.Info("Total files processed: %d", totalFiles)
	logger.Info("Unique files found: %d", uniqueFiles)
	logger.Info("Duplicate files: %d", totalFiles-uniqueFiles)
	logger.Info("Scan errors: %d", scanErrorCount)
	logger.Info("API errors: %d", apiErrorCount)
	
	fmt.Printf("\nScan completed in %s\n", elapsed)
	fmt.Printf("Total files processed: %d\n", totalFiles)
	fmt.Printf("Unique files found: %d\n", uniqueFiles)
	fmt.Printf("Duplicate files: %d\n", totalFiles-uniqueFiles)
	
	return nil
}

// splitCSV splits a comma-separated string into a slice
func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	
	var result []string
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
