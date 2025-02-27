// +build ignore

// This program validates that all the required imports can be resolved correctly.
package main

import (
	"fmt"

	// Import all packages to check if they compile
	_ "github.com/vtriple/agentflux/pkg/api"
	_ "github.com/vtriple/agentflux/pkg/common/config"
	_ "github.com/vtriple/agentflux/pkg/common/logging"
	_ "github.com/vtriple/agentflux/pkg/common/fileutils"
	_ "github.com/vtriple/agentflux/pkg/common/pathutils"
	_ "github.com/vtriple/agentflux/pkg/dedup"
	_ "github.com/vtriple/agentflux/pkg/processor"
	_ "github.com/vtriple/agentflux/pkg/scanner"
)

func main() {
	fmt.Println("All imports resolved successfully!")
}
