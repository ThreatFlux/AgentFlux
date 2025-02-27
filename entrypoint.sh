#!/bin/bash
set -e

# Setup signal handlers
trap 'kill -TERM $PID' TERM INT

# Main execution
main() {
    echo "Starting ThreatFlux AgentFlux..."
    
    # Run the main application
    exec /app/agentflux "$@" &

    # Store PID for signal handling
    PID=$!
    
    # Wait for the process to complete
    wait $PID
    
    # Capture exit code
    exit_code=$?
    
    # Exit with the same code as the main process
    exit $exit_code
}

# Run main function with all arguments passed to the script
main "$@"
