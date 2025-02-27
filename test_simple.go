package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("== AgentFlux Simple Test ==")
	
	// List the testdata directory
	testDataPath := "./testdata"
	entries, err := os.ReadDir(testDataPath)
	if err != nil {
		fmt.Printf("Error reading testdata: %v\n", err)
		return
	}
	
	fmt.Printf("Found %d entries in testdata directory:\n", len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			fmt.Printf("- Error getting info for %s: %v\n", entry.Name(), err)
			continue
		}
		
		fmt.Printf("- %s (%d bytes, dir: %v)\n", 
			entry.Name(), info.Size(), entry.IsDir())
	}
	
	// Verify package imports
	fmt.Println("\nVerifying package imports (manually)...")
	packages := []string{
		"github.com/vtriple/agentflux/pkg/api",
		"github.com/vtriple/agentflux/pkg/common/config",
		"github.com/vtriple/agentflux/pkg/common/logging",
		"github.com/vtriple/agentflux/pkg/common/fileutils",
		"github.com/vtriple/agentflux/pkg/common/pathutils",
		"github.com/vtriple/agentflux/pkg/dedup",
		"github.com/vtriple/agentflux/pkg/processor",
		"github.com/vtriple/agentflux/pkg/scanner",
	}
	
	for _, pkg := range packages {
		fmt.Printf("- %s: ", pkg)
		dir := filepath.Join(".", strings.ReplaceAll(pkg, "github.com/vtriple/agentflux/", ""))
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Println("NOT FOUND")
		} else if err != nil {
			fmt.Printf("ERROR: %v\n", err)
		} else {
			fmt.Println("OK")
		}
	}
	
	fmt.Println("\nSimple test completed!")
}
