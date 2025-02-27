package api

import "strings"

// Helper function to check if a string contains a substring (extended version)
// This version also checks that s != substr which is useful in some contexts
func containsStringExtended(s, substr string) bool {
	return s != "" && substr != "" && s != substr && strings.Contains(s, substr)
}
