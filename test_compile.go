// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	fmt.Println("Testing if the code can compile...")
	
	// List files
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading directory: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Files in current directory:")
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting file info: %v\n", err)
			continue
		}
		fmt.Printf("- %s (%d bytes)\n", entry.Name(), info.Size())
	}
	
	// Test some file operations
	testFile := filepath.Join(dir, "test_file.txt")
	data := []byte("This is a test file for AgentFlux.")
	
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}
	
	readData, err := os.ReadFile(testFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}
	
	if string(readData) != string(data) {
		fmt.Fprintf(os.Stderr, "Data mismatch!\n")
		os.Exit(1)
	}
	
	if err := os.Remove(testFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("All tests passed! Code can compile and run.")
}
