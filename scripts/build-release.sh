#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p dist
targets=(darwin-amd64 darwin-arm64 linux-amd64 linux-arm64)

for target in "${targets[@]}"; do
  CGO_ENABLED=0 GOOS="${target%-*}" GOARCH="${target#*-}" \
    go build -trimpath -o "dist/cmsg-$target" .
done

cd dist
if command -v sha256sum >/dev/null; then
  sha256sum "${targets[@]/#/cmsg-}"
else
  shasum -a 256 "${targets[@]/#/cmsg-}"
fi > SHA256SUMS
