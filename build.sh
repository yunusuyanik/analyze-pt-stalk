#!/bin/bash

# Build script for analyze-pt-stalk
# Fixes LC_UUID error on macOS by disabling CGO and using proper build flags

set -e

echo "Building analyze-pt-stalk..."

# Disable CGO to avoid LC_UUID issues on macOS
export CGO_ENABLED=0

# Build with proper flags
go build -ldflags="-s -w" -o analyze-pt-stalk main.go

echo "Build successful! Binary created: ./analyze-pt-stalk"

