#!/bin/bash
set -e

# Create a testing directory if it doesn't exist
mkdir -p /tmp/agentflux-test

# Create some test files
echo "Creating test files..."
echo "Test file 1 content" > /tmp/agentflux-test/file1.txt
echo "Test file 2 content" > /tmp/agentflux-test/file2.txt
dd if=/dev/urandom of=/tmp/agentflux-test/binary1.bin bs=1024 count=1024 2>/dev/null
mkdir -p /tmp/agentflux-test/subdir
echo "Test file in subdirectory" > /tmp/agentflux-test/subdir/nested.txt
echo "Duplicate content" > /tmp/agentflux-test/original.txt
echo "Duplicate content" > /tmp/agentflux-test/duplicate.txt

# Create a mock API server using nc or netcat if available
if command -v nc >/dev/null 2>&1; then
  echo "Starting mock API server on port 8000..."
  # Run in background
  (
    while true; do
      echo -e "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n{\"status\":\"ok\"}" | nc -l 8000
    done
  ) &
  MOCK_SERVER_PID=$!
  # Kill the mock server when this script exits
  trap "kill $MOCK_SERVER_PID" EXIT
  
  echo "Mock API server started with PID $MOCK_SERVER_PID"
  
  # Build the application
  echo "Building AgentFlux..."
  make build
  
  # Run the application with test parameters
  echo "Running AgentFlux..."
  ./build/agentflux \
    --paths=/tmp/agentflux-test \
    --algorithm=sha256 \
    --workers=2 \
    --api=http://localhost:8000 \
    --token=test-token \
    --strings \
    --string-min=4 \
    --log-level=debug
    
  echo "Test completed successfully!"
else
  echo "nc (netcat) not found - cannot start mock API server"
  echo "Building AgentFlux..."
  make build
  
  echo "Running AgentFlux in local mode (API disabled)..."
  ./build/agentflux \
    --paths=/tmp/agentflux-test \
    --algorithm=sha256 \
    --workers=2 \
    --api=http://localhost:8000 \
    --token=test-token \
    --strings \
    --string-min=4 \
    --log-level=debug
    
  echo "Test completed. Note: API was disabled due to missing netcat."
fi

# Clean up test files (optional)
# rm -rf /tmp/agentflux-test
