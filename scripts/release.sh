#!/bin/bash
set -e

# This script automates the release process for AgentFlux

# Function to display help
display_help() {
    echo "AgentFlux Release Script"
    echo ""
    echo "Usage: ./scripts/release.sh [options]"
    echo ""
    echo "Options:"
    echo "  -v, --version VERSION    Version to release (e.g., 1.0.0)"
    echo "  -d, --dry-run            Perform a dry run without making any changes"
    echo "  -h, --help               Display this help message"
    echo ""
    echo "Example:"
    echo "  ./scripts/release.sh --version 1.0.0"
}

# Parse command line arguments
DRY_RUN=false
VERSION=""

while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -v|--version)
            VERSION="$2"
            shift
            shift
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            display_help
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            display_help
            exit 1
            ;;
    esac
done

# Check if version is provided
if [ -z "$VERSION" ]; then
    echo "Error: Version is required"
    display_help
    exit 1
fi

# Validate version format (semver)
if ! [[ $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in format x.y.z (e.g., 1.0.0)"
    exit 1
fi

# Check if working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    echo "Error: Working directory is not clean. Commit or stash changes before releasing."
    exit 1
fi

# Ensure we're on the main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo "Error: Not on main branch. Please switch to main branch before releasing."
    exit 1
fi

echo "Preparing release v$VERSION..."

# Pull latest changes
echo "Pulling latest changes from remote..."
if [ "$DRY_RUN" = false ]; then
    git pull origin main
fi

# Update version in code if needed
# For this project, we don't need to update any files since version is derived from git tags

# Run tests to ensure everything is working
echo "Running tests..."
if [ "$DRY_RUN" = false ]; then
    make test
fi

# Build the project to ensure it compiles
echo "Building project..."
if [ "$DRY_RUN" = false ]; then
    make build
fi

# Create release commit
echo "Creating release commit..."
if [ "$DRY_RUN" = false ]; then
    git commit --allow-empty -m "Release v$VERSION"
fi

# Create and push tag
echo "Creating and pushing tag v$VERSION..."
if [ "$DRY_RUN" = false ]; then
    git tag -a "v$VERSION" -m "Release v$VERSION"
    git push origin main
    git push origin "v$VERSION"
    echo "Tag pushed, CI workflow will create release and build artifacts."
else
    echo "Dry run: Would create and push tag v$VERSION"
fi

# Build and push Docker image
echo "Building and pushing Docker image..."
if [ "$DRY_RUN" = false ]; then
    make docker-build
    make docker-push
    echo "Docker image pushed to registry."
else
    echo "Dry run: Would build and push Docker image vtriple/agentflux:$VERSION"
fi

echo ""
echo "Release v$VERSION completed successfully!"
if [ "$DRY_RUN" = true ]; then
    echo "Note: This was a dry run, no changes were made."
fi
