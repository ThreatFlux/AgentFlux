package pathutils

import (
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Path with forward slashes",
			path:     "/usr/local/bin/",
			expected: "/usr/local/bin",
		},
		{
			name:     "Path with backslashes",
			path:     "C:\\Windows\\System32\\",
			expected: "C:/Windows/System32",
		},
		{
			name:     "Path with mixed slashes",
			path:     "C:/Windows\\System32/",
			expected: "C:/Windows/System32",
		},
		{
			name:     "Empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "Root path with trailing slash",
			path:     "/",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizePath(tc.path)
			if result != tc.expected {
				t.Errorf("NormalizePath(%s) = %s, want %s", tc.path, result, tc.expected)
			}
		})
	}
}

func TestIsChildPath(t *testing.T) {
	tests := []struct {
		name     string
		parent   string
		child    string
		expected bool
	}{
		{
			name:     "Direct child",
			parent:   "/usr/local",
			child:    "/usr/local/bin",
			expected: true,
		},
		{
			name:     "Nested child",
			parent:   "/usr",
			child:    "/usr/local/bin",
			expected: true,
		},
		{
			name:     "Same path",
			parent:   "/usr/local",
			child:    "/usr/local",
			expected: false,
		},
		{
			name:     "Different branch",
			parent:   "/usr/local",
			child:    "/usr/share",
			expected: false,
		},
		{
			name:     "Path with similar prefix",
			parent:   "/usr/local",
			child:    "/usr/local2",
			expected: false,
		},
		{
			name:     "With Windows paths",
			parent:   "C:\\Windows",
			child:    "C:\\Windows\\System32",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsChildPath(tc.parent, tc.child)
			if result != tc.expected {
				t.Errorf("IsChildPath(%s, %s) = %v, want %v", tc.parent, tc.child, result, tc.expected)
			}
		})
	}
}

func TestIsGlobPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{
			name:     "Pattern with asterisk",
			pattern:  "*.txt",
			expected: true,
		},
		{
			name:     "Pattern with question mark",
			pattern:  "file?.txt",
			expected: true,
		},
		{
			name:     "Pattern with brackets",
			pattern:  "file[1-3].txt",
			expected: true,
		},
		{
			name:     "Pattern with braces",
			pattern:  "file{1,2,3}.txt",
			expected: true,
		},
		{
			name:     "Regular string",
			pattern:  "file.txt",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsGlobPattern(tc.pattern)
			if result != tc.expected {
				t.Errorf("IsGlobPattern(%s) = %v, want %v", tc.pattern, result, tc.expected)
			}
		})
	}
}

func TestMatchGlobPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "Simple match with asterisk",
			pattern:  "*.txt",
			path:     "file.txt",
			expected: true,
		},
		{
			name:     "Simple match with question mark",
			pattern:  "file?.txt",
			path:     "file1.txt",
			expected: true,
		},
		{
			name:     "No match with asterisk",
			pattern:  "*.txt",
			path:     "file.log",
			expected: false,
		},
		{
			name:     "Exact match",
			pattern:  "file.txt",
			path:     "file.txt",
			expected: true,
		},
		{
			name:     "Exact mismatch",
			pattern:  "file.txt",
			path:     "file1.txt",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := MatchGlobPattern(tc.pattern, tc.path)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("MatchGlobPattern(%s, %s) = %v, want %v", tc.pattern, tc.path, result, tc.expected)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "Unix path",
			path:     "/usr/local/bin",
			expected: []string{"usr", "local", "bin"},
		},
		{
			name:     "Windows path",
			path:     "C:\\Windows\\System32",
			expected: []string{"C:", "Windows", "System32"},
		},
		{
			name:     "Path with trailing slash",
			path:     "/usr/local/bin/",
			expected: []string{"usr", "local", "bin"},
		},
		{
			name:     "Relative path",
			path:     "usr/local/bin",
			expected: []string{"usr", "local", "bin"},
		},
		{
			name:     "Empty path",
			path:     "",
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SplitPath(tc.path)
			
			// Check length
			if len(result) != len(tc.expected) {
				t.Errorf("SplitPath(%s) returned %d components, want %d", tc.path, len(result), len(tc.expected))
				return
			}
			
			// Check each component
			for i, component := range result {
				if component != tc.expected[i] {
					t.Errorf("SplitPath(%s)[%d] = %s, want %s", tc.path, i, component, tc.expected[i])
				}
			}
		})
	}
}

func TestEscapeRegExp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "String with special characters",
			input:    "file*.txt",
			expected: "file\\*\\.txt",
		},
		{
			name:     "String with multiple special characters",
			input:    "^file[0-9].txt$",
			expected: "\\^file\\[0-9\\]\\.txt\\$",
		},
		{
			name:     "String without special characters",
			input:    "file",
			expected: "file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := EscapeRegExp(tc.input)
			if result != tc.expected {
				t.Errorf("EscapeRegExp(%s) = %s, want %s", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGlobPatternToRegExp(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		testString    string
		shouldMatch   bool
	}{
		{
			name:          "Simple asterisk pattern",
			pattern:       "*.txt",
			testString:    "file.txt",
			shouldMatch:   true,
		},
		{
			name:          "Double asterisk pattern",
			pattern:       "**/file.txt",
			testString:    "dir/subdir/file.txt",
			shouldMatch:   true,
		},
		{
			name:          "Question mark pattern",
			pattern:       "file?.txt",
			testString:    "file1.txt",
			shouldMatch:   true,
		},
		{
			name:          "Non-matching pattern",
			pattern:       "*.txt",
			testString:    "file.log",
			shouldMatch:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			re, err := GlobPatternToRegExp(tc.pattern)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			result := re.MatchString(tc.testString)
			if result != tc.shouldMatch {
				t.Errorf("GlobPatternToRegExp(%s).MatchString(%s) = %v, want %v", 
					tc.pattern, tc.testString, result, tc.shouldMatch)
			}
		})
	}
}
