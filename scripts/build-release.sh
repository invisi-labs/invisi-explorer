#!/usr/bin/env bash
set -euo pipefail

# Run from the checked-out release source with Go installed.
tag=${1:?Usage: scripts/build-release.sh vX.Y.Z [output-directory]}
output=${2:-dist}
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo 'Expected a stable version tag such as v1.2.10' >&2
  exit 1
fi
python3 scripts/release-notes.py "$tag" >/dev/null
python3 - "$tag" <<'PYVERSION'
import json, pathlib, sys
version = sys.argv[1][1:]
assert json.loads(pathlib.Path("web/package.json").read_text())["version"] == version
assert json.loads(pathlib.Path("web/package-lock.json").read_text())["version"] == version
assert f'Version: "{version}"' in pathlib.Path("cmd/invisi-explorer/main.go").read_text()
PYVERSION
(cd web && npm ci && npm run build)
mkdir -p "$output"
output=$(cd "$output" && pwd)
staging=$(mktemp -d)
trap 'rm -rf "$staging"' EXIT
for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  target_os=${platform%/*}
  target_arch=${platform#*/}
  name="invisi-explorer_${tag}_${target_os}_${target_arch}"
  package="$staging/$name"
  mkdir -p "$package"
  binary=invisi-explorer
  GOWORK=off CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
    go build -trimpath -o "$package/$binary" ./cmd/invisi-explorer
  cp LICENSE README.md api.md cammand.md CHANGELOG.md "$package/"
  mkdir -p "$package/web"
  cp web/README.md "$package/web/"
  cp -R web/dist "$package/web/"
  COPYFILE_DISABLE=1 tar -czf "$output/$name.tar.gz" -C "$staging" "$name"
  echo "Built $name"
done
# Restrict checksums to this release when reusing the output directory.
(
  cd "$output"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum invisi-explorer_"$tag"_*.tar.gz
  else
    shasum -a 256 invisi-explorer_"$tag"_*.tar.gz
  fi
) > "$output/SHA256SUMS-$tag.txt"
