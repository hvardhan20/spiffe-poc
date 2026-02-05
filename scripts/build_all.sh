#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/lambda/dist"
mkdir -p "$DIST"

echo "==> Building extension layer..."
bash "$ROOT/scripts/build_extension_layer.sh"

echo "==> Building authorizer zip..."
bash "$ROOT/scripts/build_authorizer.sh"

echo "==> Building business zip..."
bash "$ROOT/scripts/build_business.sh"

echo "Done. Artifacts in: $DIST"
ls -lh "$DIST"
