#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/lambda/dist"
WORK="$ROOT/lambda/.build_business"
rm -rf "$WORK"
mkdir -p "$WORK" "$DIST"

pushd "$ROOT/lambda/business" >/dev/null
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$WORK/bootstrap" .
popd >/dev/null

pushd "$WORK" >/dev/null
zip -qr "$DIST/business.zip" bootstrap
popd >/dev/null

echo "Built: $DIST/business.zip"
