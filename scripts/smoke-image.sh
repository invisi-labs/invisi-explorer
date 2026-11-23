#!/usr/bin/env bash
set -euo pipefail
image=${1:?Usage: scripts/smoke-image.sh IMAGE}
volume=$(docker volume create)
container=
cleanup() {
  if [[ -n "$container" ]]; then docker rm -f "$container" >/dev/null; fi
  docker volume rm "$volume" >/dev/null
}
trap cleanup EXIT
docker run --rm "$image" --version
docker run --rm -v "$volume:/var/lib/invisi-explorer" "$image" init
docker run --rm -v "$volume:/var/lib/invisi-explorer" "$image" settings set --web-dir /opt/invisi/explorer
container=$(docker run -d -p 127.0.0.1::18670 -v "$volume:/var/lib/invisi-explorer" "$image")
port=$(docker port "$container" 18670/tcp)
python3 scripts/smoke-http.py "http://$port"
staging=$(mktemp -d)
docker cp "$container:/licenses/invisi-explorer/LICENSE" "$staging/LICENSE"
cmp LICENSE "$staging/LICENSE"
rm -rf "$staging"
echo 'Verified image CLI, static HTTP serving, logo, font and LICENSE'
