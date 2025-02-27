// build.go provides a replacement for the full build in integration tests
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BuildForTest builds a minimal version of the application for testing
func BuildForTest() (string, error) {
	// Get the project root directory (3 levels up from integration package)
	_, currentFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..")
	
	// Create the build directory if it doesn't exist
	buildDir := filepath.Join(projectRoot, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", err
	}
	
	// Build path for the binary
	binaryPath := filepath.Join(buildDir, "agentflux")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	
	// Check if the binary already exists and is recent
	if _, err := os.Stat(binaryPath); err == nil {
		// Binary exists, use it
		return binaryPath, nil
	}
	
	// Build the application
	cmd := exec.Command("go", "build", "-o", binaryPath, filepath.Join(projectRoot, "cmd", "agentflux"))
	cmd.Dir = projectRoot
	
	// Run the build command
	if err := cmd.Run(); err != nil {
		return "", err
	}
	
	return binaryPath, nil
}
