#!/bin/bash
set -e

echo "Setting up Git hooks..."

# Check if running from the project root
if [ ! -d ".git" ]; then
    echo "Error: This script must be run from the project root directory."
    exit 1
fi

# Create the githooks directory if it doesn't exist
mkdir -p .githooks

# Make git hooks executable
chmod +x .githooks/*

# Configure git to use the githooks directory
git config core.hooksPath .githooks

echo "Git hooks have been set up successfully."
echo "The following hooks are now active:"
ls -la .githooks/
