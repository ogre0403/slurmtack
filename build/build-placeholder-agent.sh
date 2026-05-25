#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/output"

mkdir -p "$OUTPUT_DIR"

echo "==> Building placeholder-agent (static binary via Docker)..."
docker run --rm \
  -v "$PROJECT_ROOT":/src \
  -v "$OUTPUT_DIR":/out \
  -w /src \
  -e CGO_ENABLED=0 \
  -e GOOS=linux \
  -e GOARCH=amd64 \
  golang:1.22.4-alpine \
  go build -o /out/placeholder-agent ./cmd/placeholder-agent/

echo "    Binary: $OUTPUT_DIR/placeholder-agent"

if command -v singularity &>/dev/null; then
  echo "==> Building Singularity SIF image..."
  cd "$PROJECT_ROOT"
  singularity build "$OUTPUT_DIR/placeholder-agent.sif" "$SCRIPT_DIR/placeholder-agent.def"
  echo "    SIF: $OUTPUT_DIR/placeholder-agent.sif"
  echo ""
  echo "==> Deployment:"
  echo "    Copy the SIF to a shared filesystem path accessible by all GPU nodes."
  echo "    Set PLACEHOLDER_SIF_PATH in the daemon config to point to the SIF location."
  echo "    Example: /shared/images/placeholder-agent.sif"
else
  echo ""
  echo "WARNING: singularity not found. SIF image build skipped."
  echo "         Install Singularity to build the container image."
  echo "         The binary can still be used directly for testing."
fi
