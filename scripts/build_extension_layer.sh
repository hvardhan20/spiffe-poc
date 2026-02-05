#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/lambda/dist"
WORK="$ROOT/lambda/.build_layer"
rm -rf "$WORK"
mkdir -p "$WORK/extensions" "$DIST"

pushd "$ROOT/lambda/extension" >/dev/null
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$WORK/extensions/spiffe-verifier-extension" .
popd >/dev/null

chmod +x "$WORK/extensions/spiffe-verifier-extension"

pushd "$WORK" >/dev/null
zip -qr "$DIST/extension-layer.zip" .
popd >/dev/null

echo "Built: $DIST/extension-layer.zip"
