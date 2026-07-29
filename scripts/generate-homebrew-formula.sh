#!/usr/bin/env bash

set -euo pipefail

TAG="${1:?release tag is required}"
DIST_DIR="${2:?distribution directory is required}"

if [[ "$TAG" != v* || "$TAG" == "v" ]]; then
  echo "release tag must start with v and include a version" >&2
  exit 1
fi

VERSION="${TAG#v}"

sha256() {
  local artifact="$DIST_DIR/$1"
  if [[ ! -f "$artifact" ]]; then
    echo "missing release artifact: $artifact" >&2
    exit 1
  fi

  shasum -a 256 "$artifact" | awk '{print $1}'
}

DARWIN_ARM64_SHA="$(sha256 "pxon_${VERSION}_darwin_arm64.zip")"
DARWIN_AMD64_SHA="$(sha256 "pxon_${VERSION}_darwin_amd64.zip")"
LINUX_ARM64_SHA="$(sha256 "pxon_${VERSION}_linux_arm64.tar.gz")"
LINUX_AMD64_SHA="$(sha256 "pxon_${VERSION}_linux_amd64.tar.gz")"
BASE_URL="https://github.com/illegalstudio/pxon/releases/download/${TAG}"

cat <<EOF
class Pxon < Formula
  desc "Create and manage Proxmox VE LXC containers"
  homepage "https://github.com/illegalstudio/pxon"
  version "${VERSION}"

  on_macos do
    on_arm do
      url "${BASE_URL}/pxon_${VERSION}_darwin_arm64.zip"
      sha256 "${DARWIN_ARM64_SHA}"
    end
    on_intel do
      url "${BASE_URL}/pxon_${VERSION}_darwin_amd64.zip"
      sha256 "${DARWIN_AMD64_SHA}"
    end
  end

  on_linux do
    on_arm do
      url "${BASE_URL}/pxon_${VERSION}_linux_arm64.tar.gz"
      sha256 "${LINUX_ARM64_SHA}"
    end
    on_intel do
      url "${BASE_URL}/pxon_${VERSION}_linux_amd64.tar.gz"
      sha256 "${LINUX_AMD64_SHA}"
    end
  end

  def install
    bin.install "pxon"
  end

  test do
    assert_match "pxon v#{version}", shell_output("#{bin}/pxon --version")
  end
end
EOF
