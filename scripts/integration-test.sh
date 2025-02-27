#!/bin/bash
set -e

echo "Running integration tests for AgentFlux..."

# Set up environment
WORKDIR=$(pwd)
TEST_DIR=/tmp/agentflux-integration-test
API_PORT=8765
LOGS_DIR=$WORKDIR/integration-logs
RESULTS_FILE=$LOGS_DIR/test_results.txt

# Create directories
mkdir -p $TEST_DIR
mkdir -p $LOGS_DIR

# Function to clean up processes
cleanup() {
  echo "Cleaning up..."
  
  # Kill mock API server if running
  if [ -n "$API_PID" ]; then
    kill $API_PID 2>/dev/null || true
    echo "Mock API server stopped"
  fi
  
  # Optional: Remove test directory
  # rm -rf $TEST_DIR
  
  echo "Cleanup complete"
}

# Register the cleanup function to be called on exit
trap cleanup EXIT

# Create test files
echo "Creating test files..."
# Standard text files
echo "Test file 1 content" > $TEST_DIR/file1.txt
echo "Test file 2 content" > $TEST_DIR/file2.txt

# Binary files
dd if=/dev/urandom of=$TEST_DIR/binary1.bin bs=1024 count=10 2>/dev/null
dd if=/dev/urandom of=$TEST_DIR/binary2.bin bs=1024 count=100 2>/dev/null

# Create subdirectories with files
mkdir -p $TEST_DIR/subdir1/subdir2
echo "Nested file content" > $TEST_DIR/subdir1/subdir2/nested.txt

# Create duplicate files
echo "Duplicate content" > $TEST_DIR/original.txt
echo "Duplicate content" > $TEST_DIR/duplicate.txt

# Create hidden files
echo "Hidden file content" > $TEST_DIR/.hidden

# Create symlinks if supported
if [ "$(uname)" != "MINGW"* ] && [ "$(uname)" != "MSYS"* ]; then
  ln -sf $TEST_DIR/file1.txt $TEST_DIR/symlink.txt
fi

echo "Test files created at $TEST_DIR"

# Start mock API server
echo "Starting mock API server on port $API_PORT..."
{
  while true; do
    {
      echo -e "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n{\"status\":\"ok\"}"
    } | nc -l $API_PORT 2>/dev/null || true
  done
} &
API_PID=$!
echo "Mock API server started with PID $API_PID"

# Wait for the mock API server to start
sleep 1

# Build the application
echo "Building AgentFlux..."
make build &> $LOGS_DIR/build.log || {
  echo "Build failed. Check $LOGS_DIR/build.log for details."
  exit 1
}

# Run the tests
echo "Running integration tests..."
echo "========================" > $RESULTS_FILE
echo "Integration Test Results" >> $RESULTS_FILE
echo "========================" >> $RESULTS_FILE
echo "" >> $RESULTS_FILE

# Test 1: Basic scan
echo "Test 1: Basic scan"
./build/agentflux \
  --paths=$TEST_DIR \
  --algorithm=sha256 \
  --workers=2 \
  --api=http://localhost:$API_PORT \
  --token=test-token \
  &> $LOGS_DIR/test1.log || {
    echo "Test 1 failed. Check $LOGS_DIR/test1.log for details."
    echo "Test 1: Basic scan - FAILED" >> $RESULTS_FILE
    exit 1
  }
echo "Test 1: Basic scan - PASSED" >> $RESULTS_FILE

# Test 2: With string extraction
echo "Test 2: With string extraction"
./build/agentflux \
  --paths=$TEST_DIR \
  --algorithm=sha256 \
  --workers=2 \
  --api=http://localhost:$API_PORT \
  --token=test-token \
  --strings \
  --string-min=4 \
  &> $LOGS_DIR/test2.log || {
    echo "Test 2 failed. Check $LOGS_DIR/test2.log for details."
    echo "Test 2: With string extraction - FAILED" >> $RESULTS_FILE
    exit 1
  }
echo "Test 2: With string extraction - PASSED" >> $RESULTS_FILE

# Test 3: With exclusion patterns
echo "Test 3: With exclusion patterns"
./build/agentflux \
  --paths=$TEST_DIR \
  --exclude="*.bin,*.hidden" \
  --algorithm=sha256 \
  --workers=2 \
  --api=http://localhost:$API_PORT \
  --token=test-token \
  &> $LOGS_DIR/test3.log || {
    echo "Test 3 failed. Check $LOGS_DIR/test3.log for details."
    echo "Test 3: With exclusion patterns - FAILED" >> $RESULTS_FILE
    exit 1
  }
echo "Test 3: With exclusion patterns - PASSED" >> $RESULTS_FILE

# Test 4: With maximum depth
echo "Test 4: With maximum depth"
./build/agentflux \
  --paths=$TEST_DIR \
  --depth=1 \
  --algorithm=sha256 \
  --workers=2 \
  --api=http://localhost:$API_PORT \
  --token=test-token \
  &> $LOGS_DIR/test4.log || {
    echo "Test 4 failed. Check $LOGS_DIR/test4.log for details."
    echo "Test 4: With maximum depth - FAILED" >> $RESULTS_FILE
    exit 1
  }
echo "Test 4: With maximum depth - PASSED" >> $RESULTS_FILE

# Test 5: Different hash algorithm
echo "Test 5: Different hash algorithm"
./build/agentflux \
  --paths=$TEST_DIR \
  --algorithm=md5 \
  --workers=2 \
  --api=http://localhost:$API_PORT \
  --token=test-token \
  &> $LOGS_DIR/test5.log || {
    echo "Test 5 failed. Check $LOGS_DIR/test5.log for details."
    echo "Test 5: Different hash algorithm - FAILED" >> $RESULTS_FILE
    exit 1
  }
echo "Test 5: Different hash algorithm - PASSED" >> $RESULTS_FILE

# Print summary
echo ""
echo "Integration tests completed successfully."
echo "Results are available at $RESULTS_FILE"
echo ""
cat $RESULTS_FILE
