#!/bin/bash

# Build script for Lambda functions
# This script builds Go Lambda functions for deployment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LAMBDA_SRC_DIR="$(dirname "$SCRIPT_DIR")/lambda-src"

echo "Building Lambda functions..."
echo "Source directory: $LAMBDA_SRC_DIR"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to build a Lambda function
build_lambda() {
    local service_name=$1
    local service_dir="$LAMBDA_SRC_DIR/$service_name"

    if [ ! -d "$service_dir" ]; then
        echo -e "${RED}✗ Directory not found: $service_dir${NC}"
        return 1
    fi

    echo -e "\n📦 Building $service_name..."

    cd "$service_dir"

    # Download dependencies
    echo "  → Running go mod download..."
    go mod download

    # Build for Linux (Lambda runtime)
    echo "  → Building binary for Linux/amd64..."
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bootstrap main.go

    if [ $? -eq 0 ]; then
        # Get file size
        size=$(ls -lh bootstrap | awk '{print $5}')
        echo -e "${GREEN}✓ Built $service_name successfully ($size)${NC}"
    else
        echo -e "${RED}✗ Failed to build $service_name${NC}"
        return 1
    fi
}

# Build all Lambda functions
services=(
    "user-service"
    "order-service"
    "admin-service"
    "status-service"
)

failed_builds=()

for service in "${services[@]}"; do
    if ! build_lambda "$service"; then
        failed_builds+=("$service")
    fi
done

echo ""
echo "=========================================="
echo "Build Summary"
echo "=========================================="

if [ ${#failed_builds[@]} -eq 0 ]; then
    echo -e "${GREEN}✓ All Lambda functions built successfully!${NC}"
    exit 0
else
    echo -e "${RED}✗ Failed to build the following functions:${NC}"
    for service in "${failed_builds[@]}"; do
        echo "  - $service"
    done
    exit 1
fi
