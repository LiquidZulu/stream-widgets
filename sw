#!/usr/bin/env bash

# Exit on any error
set -e

# Get the absolute path of the script's directory
ORIGIN_DIR="$(pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"

# Function to clean up background processes on exit
cleanup() {
    echo "Shutting down..."
    # Kill all background jobs (wrangler and go server)
    kill $(jobs -p) 2>/dev/null || true
    cd $ORIGIN_DIR
    exit 0
}

# Trap Ctrl+C and EXIT to run cleanup
trap cleanup SIGINT SIGTERM EXIT

# Check if Node.js is installed
if ! command -v node >/dev/null 2>&1; then
    echo "Error: Node.js is not installed. Please install Node.js to run wrangler."
    exit 1
fi

# Check if wrangler is installed
if ! command -v wrangler >/dev/null 2>&1; then
    echo "Error: wrangler is not installed. Please install wrangler to run the server."
    exit 1
fi

# Check if Go is installed
if ! command -v go >/dev/null 2>&1; then
    echo "Error: Go is not installed. Please install Go to run the server."
    exit 1
fi

# Check if youtube-websocket submodule exists
if [ ! -d "$PROJECT_ROOT/submodules/youtube-websocket" ]; then
    echo "Error: youtube-websocket submodule not found."
    exit 1
fi

# Navigate to youtube-websocket submodule and install dependencies
cd "$PROJECT_ROOT/submodules/youtube-websocket"
if [ ! -d "node_modules" ]; then
    echo "Installing youtube-websocket dependencies..."
    npm install
fi

# Start wrangler dev in the background
echo "Starting youtube-websocket Worker..."
wrangler dev &
WRANGLER_PID=$!
# Wait briefly to ensure wrangler starts
sleep 0.5
# Check if wrangler is running
if ! ps -p $WRANGLER_PID >/dev/null; then
    echo "Error: Failed to start wrangler. Check logs above for details."
    exit 1
fi

cd $PROJECT_ROOT

# Start the Go server
echo "Starting Go server..."
go run server.go
