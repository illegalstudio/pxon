#!/usr/bin/env bash

set -euo pipefail

BINARY="${1:?binary path is required}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENTITLEMENTS="$SCRIPT_DIR/entitlements.plist"

if [[ -z "${APPLE_SIGNING_IDENTITY:-}" ]]; then
  echo "APPLE_SIGNING_IDENTITY is not set; skipping code signing"
  exit 0
fi

echo "Signing $BINARY ..."
codesign --force --options runtime \
  --sign "$APPLE_SIGNING_IDENTITY" \
  --entitlements "$ENTITLEMENTS" \
  --timestamp \
  "$BINARY"

codesign --verify --verbose "$BINARY"
