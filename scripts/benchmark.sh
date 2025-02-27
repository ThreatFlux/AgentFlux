#!/bin/bash
set -e

# This script benchmarks AgentFlux with different configurations

# Create benchmark directory if it doesn't exist
BENCHMARK_DIR="/tmp/agentflux-benchmark"
mkdir -p "$BENCHMARK_DIR"

# Generate test files of various sizes if they don't exist
if [ ! -f "$BENCHMARK_DIR/1kb.bin" ]; then
  echo "Generating benchmark files..."
  for size in 1 10 100 1000; do
    dd if=/dev/urandom of="$BENCHMARK_DIR/${size}kb.bin" bs=1024 count=$size 2>/dev/null
  done
  
  # Create a directory with many small files
  mkdir -p "$BENCHMARK_DIR/many_files"
  for i in {1..100}; do
    echo "File $i content" > "$BENCHMARK_DIR/many_files/file_$i.txt"
  done
  
  # Create a nested directory structure
  mkdir -p "$BENCHMARK_DIR/nested/level1/level2/level3"
  for level in nested nested/level1 nested/level1/level2 nested/level1/level2/level3; do
    for i in {1..10}; do
      echo "Nested file $i at $level" > "$BENCHMARK_DIR/$level/file_$i.txt"
    done
  done
  
  echo "Benchmark files generated."
fi

# Build the application
echo "Building AgentFlux..."
make build

# Define benchmark configurations
CONFIGS=(
  "Default:--workers=$(nproc):--algorithm=sha256"
  "MoreWorkers:--workers=$(($(nproc)*2)):--algorithm=sha256"
  "FewerWorkers:--workers=2:--algorithm=sha256"
  "MD5:--workers=$(nproc):--algorithm=md5"
  "SHA512:--workers=$(nproc):--algorithm=sha512"
  "WithStrings:--workers=$(nproc):--algorithm=sha256:--strings"
)

# Run benchmarks
echo "Running benchmarks..."
echo "======================="
echo "Configuration | Time (s) | Files/sec"
echo "------------- | -------- | ---------"

for config in "${CONFIGS[@]}"; do
  # Parse config
  IFS=':' read -r name workers algorithm strings <<< "$config"
  
  # Construct command
  cmd="./build/agentflux --paths=$BENCHMARK_DIR $workers $algorithm $strings --api=http://localhost:8000 --token=benchmark-token"
  
  # Count total files
  total_files=$(find "$BENCHMARK_DIR" -type f | wc -l)
  
  # Run benchmark with time measurement
  start_time=$(date +%s.%N)
  $cmd >/dev/null 2>&1 || echo "Benchmark failed for $name configuration"
  end_time=$(date +%s.%N)
  
  # Calculate elapsed time and files per second
  elapsed=$(echo "$end_time - $start_time" | bc)
  files_per_sec=$(echo "scale=2; $total_files / $elapsed" | bc)
  
  # Print results
  printf "%-13s | %-8.2f | %-8.2f\n" "$name" "$elapsed" "$files_per_sec"
done

echo "======================="
echo "Benchmark completed."
