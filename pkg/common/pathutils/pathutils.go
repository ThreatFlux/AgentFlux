// Package pathutils provides utility functions for path operations and manipulations.
package pathutils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// NormalizePath converts a path to a consistent format regardless of OS.
// Converts backslashes to forward slashes for cross-platform compatibility.
func NormalizePath(path string) string {
	// Replace backslashes with forward slashes
	normalized := strings.ReplaceAll(path, "\\", "/")
	
	// Special case for root path "/"
	if normalized == "/" {
		return ""
	}
	
	// Remove trailing slash if present
	if len(normalized) > 1 && strings.HasSuffix(normalized, "/") {
		normalized = normalized[:len(normalized)-1]
	}
	
	return normalized
}

// IsChildPath checks if child is a child/descendant path of parent.
func IsChildPath(parent, child string) bool {
	// Normalize both paths to ensure consistent comparison
	normalizedParent := NormalizePath(parent)
	normalizedChild := NormalizePath(child)
	
	// Special case for root path
	if normalizedParent == "" {
		return true
	}
	
	// Check if child starts with parent path
	if !strings.HasPrefix(normalizedChild, normalizedParent) {
		return false
	}
	
	// If parent is exactly equal to child, it's not a child path
	if normalizedParent == normalizedChild {
		return false
	}
	
	// If parent doesn't end with a slash, ensure the next character in child is a slash
	if !strings.HasSuffix(normalizedParent, "/") {
		// If the length of normalizedChild is greater than the length of normalizedParent
		// and the next character is not a slash, it's not a proper child path
		if len(normalizedChild) > len(normalizedParent) && 
		   normalizedChild[len(normalizedParent)] != '/' {
			return false
		}
	}
	
	return true
}

// IsGlobPattern checks if a string contains glob pattern characters.
func IsGlobPattern(pattern string) bool {
	for _, c := range pattern {
		switch c {
		case '*', '?', '[', '{':
			return true
		}
	}
	return false
}

// EscapeRegExp escapes special characters in a string for use in a regular expression.
func EscapeRegExp(s string) string {
	special := []string{"\\", ".", "+", "*", "?", "(", ")", "[", "]", "{", "}", "^", "$", "|"}
	for _, c := range special {
		s = strings.ReplaceAll(s, c, "\\"+c)
	}
	return s
}

// GlobPatternToRegExp converts a glob pattern to a regular expression.
func GlobPatternToRegExp(pattern string) (*regexp.Regexp, error) {
	// Escape special characters for RegExp
	regexpPattern := EscapeRegExp(pattern)
	
	// Replace glob patterns with RegExp equivalents
	regexpPattern = strings.ReplaceAll(regexpPattern, "\\*\\*", ".*")  // ** matches any characters
	regexpPattern = strings.ReplaceAll(regexpPattern, "\\*", "[^/]*")  // * matches any characters except /
	regexpPattern = strings.ReplaceAll(regexpPattern, "\\?", "[^/]")   // ? matches a single character except /
	
	// Ensure the pattern matches the entire string
	regexpPattern = "^" + regexpPattern + "$"
	
	// Compile the regular expression
	return regexp.Compile(regexpPattern)
}

// MatchGlobPattern checks if a path matches a glob pattern.
func MatchGlobPattern(pattern, path string) (bool, error) {
	// Try simple matching first
	matched, err := filepath.Match(pattern, path)
	if err == nil && matched {
		return true, nil
	}
	
	// If it's not a simple pattern or didn't match, use regular expressions
	if IsGlobPattern(pattern) {
		re, err := GlobPatternToRegExp(pattern)
		if err != nil {
			return false, err
		}
		return re.MatchString(path), nil
	}
	
	return false, nil
}

// SplitPath splits a path into its components.
func SplitPath(path string) []string {
	// First normalize the path for consistent handling
	normalized := NormalizePath(path)
	
	// Handle empty path
	if normalized == "" {
		return []string{}
	}
	
	// Split by forward slash
	parts := strings.Split(normalized, "/")
	
	// Filter out empty components that might result from trailing slashes
	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	
	return result
}

// RelativePath returns the relative path from base to target.
func RelativePath(base, target string) (string, error) {
	return filepath.Rel(base, target)
}
