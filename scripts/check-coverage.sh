#!/bin/bash
set -e

COVERAGE_THRESHOLD=95.0

echo "Running tests with coverage..."
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

echo ""
echo "Coverage by package:"
go tool cover -func=coverage.out

TOTAL_COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')

echo ""
echo "Total coverage: ${TOTAL_COVERAGE}%"
echo "Threshold: ${COVERAGE_THRESHOLD}%"

if (( $(echo "$TOTAL_COVERAGE < $COVERAGE_THRESHOLD" | bc -l) )); then
    echo "❌ Coverage is below threshold!"
    exit 1
else
    echo "✅ Coverage meets threshold!"
    exit 0
fi
