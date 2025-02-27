// Package config provides configuration structures and utilities.
package config

// Config holds the application configuration.
type Config struct {
	// File scanning options
	RootPaths        string   // Comma-separated list of paths to scan
	ParsedRootPaths  []string // Parsed paths
	ExcludePaths     string   // Comma-separated list of glob patterns to exclude
	ParsedExcludePaths []string // Parsed exclude patterns
	MaxDepth         int      // Maximum directory depth (-1 for unlimited)
	MaxFileSize      int64    // Maximum file size to process in bytes

	// Hash processing options
	HashAlgorithm    string   // Hash algorithm (md5, sha1, sha256, sha512)
	WorkerCount      int      // Number of worker goroutines
	ExtractStrings   bool     // Whether to extract strings from files
	StringMinLength  int      // Minimum string length to extract

	// API options
	APIEndpoint      string   // API endpoint URL
	APIToken         string   // API authentication token
	APIAuthMethod    string   // API authentication method
	APIBatchSize     int      // API batch size

	// Logging options
	LogLevel         string   // Log level (debug, info, warn, error)
	LogFile          string   // Path to log file (empty for stderr)

	// Misc options
	ShowVersion      bool     // Show version information
}
