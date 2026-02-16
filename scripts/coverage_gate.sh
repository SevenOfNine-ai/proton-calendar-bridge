#!/usr/bin/env bash
set -euo pipefail

threshold="${1:-85}"
go test ./... -coverprofile=coverage.out

total=$(go tool cover -func=coverage.out | awk '/total:/ {gsub("%", "", $3); print $3}')
echo "Total coverage: ${total}%"
awk -v total="$total" -v threshold="$threshold" 'BEGIN {if (total+0 < threshold+0) {exit 1}}'
