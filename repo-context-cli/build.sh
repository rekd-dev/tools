#!/usr/bin/env bash
# Build script for Linux/macOS — outputs to repo bin/ directory
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
bin_dir="$root/bin"
out_path="$bin_dir/repo-context"

if ! command -v go >/dev/null 2>&1; then
  echo "Go is not installed or not in PATH. Install Go from https://golang.org/dl/" >&2
  exit 1
fi

echo "Go installed: $(go version)"
mkdir -p "$bin_dir"
go build -ldflags="-s -w" -o "$out_path"
echo "Build successful: $out_path"
