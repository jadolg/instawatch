#!/usr/bin/env bash
# Starts the given image, downloads a real public Instagram reel through it,
# and checks that a valid mp4 comes back. Used by .github/workflows/docker.yml
# after each arch's image is built.
set -euo pipefail

IMAGE="${1:?usage: e2e-smoke-test.sh <image>}"
PORT="${2:-18080}"
CONTAINER_NAME="instawatch-e2e-$$"
TEST_URL="https://www.instagram.com/reel/DZdHKycNrsb/"

cleanup() {
  echo "::group::container logs"
  docker logs "$CONTAINER_NAME" 2>&1 || true
  echo "::endgroup::"
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --name "$CONTAINER_NAME" -p "${PORT}:8080" "$IMAGE"

echo "Waiting for InstaWatch to come up..."
ready=""
for i in $(seq 1 30); do
  if curl -sf "http://localhost:${PORT}/" >/dev/null; then
    echo "Ready after ${i}s"
    ready=1
    break
  fi
  sleep 1
done
if [ -z "$ready" ]; then
  echo "::error::service did not become ready within 30s"
  exit 1
fi

echo "Requesting player page for a real Instagram reel..."
page="$(mktemp)"
status=$(curl -sfL -o "$page" -w '%{http_code}' --max-time 90 "http://localhost:${PORT}/${TEST_URL}")
if [ "$status" != "200" ]; then
  echo "::error::player page returned HTTP $status"
  cat "$page"
  exit 1
fi

hash=$(grep -oE '/video/[0-9a-f]{32}' "$page" | head -1 | sed 's#/video/##')
if [ -z "$hash" ]; then
  echo "::error::could not find a video hash in the player page"
  cat "$page"
  exit 1
fi

echo "Downloading the served video (hash=${hash})..."
video="$(mktemp --suffix=.mp4)"
video_status=$(curl -sf -o "$video" -w '%{http_code}' --max-time 60 "http://localhost:${PORT}/video/${hash}")
if [ "$video_status" != "200" ]; then
  echo "::error::video endpoint returned HTTP $video_status"
  exit 1
fi

size=$(stat -c%s "$video")
echo "Downloaded ${size} bytes"
if [ "$size" -lt 100000 ]; then
  echo "::error::downloaded video is suspiciously small (${size} bytes)"
  exit 1
fi

if ! file "$video" | grep -qi 'ISO Media\|MP4'; then
  echo "::error::downloaded file does not look like an mp4"
  file "$video"
  exit 1
fi

echo "End-to-end check passed: real Instagram reel downloaded and served correctly."
