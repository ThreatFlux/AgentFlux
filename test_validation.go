// This file is used to test if the codebase compiles properly
package main

import (
	"fmt"
	"github.com/vtriple/agentflux/pkg/common/fileutils"
	"github.com/vtriple/agentflux/pkg/common/pathutils"
)

func main() {
	// Test fileutils
	hidden := fileutils.IsHiddenFile("/path/to/.hidden")
	fmt.Println("Is hidden file:", hidden)

	// Test pathutils
	normalized := pathutils.NormalizePath("/path/with/trailing/slash/")
	fmt.Println("Normalized path:", normalized)

	fmt.Println("Compilation test passed!")
}
