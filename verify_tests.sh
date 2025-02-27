#!/bin/bash

# Script to verify test files in AgentFlux project

echo "Verifying test files in AgentFlux project..."

# Base path for the project
BASE_PATH="/Users/vtriple/agentflux"

# Check if each package has a corresponding test file
function check_package() {
    pkg=$1
    echo -n "Checking $pkg... "
    
    # Get all Go files in the package
    go_files=$(find "$BASE_PATH/$pkg" -name "*.go" ! -name "*_test.go" 2>/dev/null)
    test_files=$(find "$BASE_PATH/$pkg" -name "*_test.go" 2>/dev/null)
    
    if [ -z "$go_files" ]; then
        echo "NO GO FILES"
        return
    fi
    
    if [ -z "$test_files" ]; then
        echo "NO TEST FILES"
        return
    fi
    
    echo "OK ($(echo "$test_files" | wc -l | tr -d ' ') test files)"
}

# Check main packages
echo "Core packages:"
check_package "pkg/api"
check_package "pkg/common/config"
check_package "pkg/common/logging"
check_package "pkg/common/fileutils"
check_package "pkg/common/pathutils"
check_package "pkg/dedup"
check_package "pkg/processor"
check_package "pkg/scanner"
check_package "pkg/integration"

echo ""
echo "Test data files:"
find "$BASE_PATH/testdata" -type f | while read file; do
    size=$(stat -f%z "$file" 2>/dev/null || echo "unknown")
    echo "- $(basename "$file") ($size bytes)"
done

echo ""
echo "Verification complete!"
