#!/usr/bin/env bash
set -euo pipefail
just _check-submodule
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
echo "Building jive-vocals version: $VERSION"
CGO_ENABLED=1 go build -ldflags="-X main.version=$VERSION" -o jive-vocals ./cmd/jive-vocals
