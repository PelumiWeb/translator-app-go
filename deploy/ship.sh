#!/bin/bash
#
# Builds the server for the host (Linux), copies it over SSH as the deploy
# user, restarts it, and waits until it answers over HTTPS. The server
# applies new migrations as it starts. GitHub Actions runs this on every push
# to main that changes the server; on a Mac:
#
#   SERVER_HOST=shinar-api.southafricanorth.cloudapp.azure.com \
#   SERVER_SSH_KEY_PATH=~/.ssh/shinar_deploy make server-ship
#
# The program it replaces stays beside it as shinar.previous, to go back to
# by hand. ADR 0023.
set -euo pipefail

HOST=${SERVER_HOST:?the server host name}
# The Azure size chosen, B2ats v2, is an x86-64 (AMD) machine.
ARCH=${SERVER_ARCH:-amd64}
ROOT=$(cd "$(dirname "$0")/.." && pwd)

ssh_options=(-o BatchMode=yes)
if [[ -n ${SERVER_SSH_KEY_PATH:-} ]]; then
    ssh_options+=(-i "$SERVER_SSH_KEY_PATH")
fi

build=$(mktemp -d)
trap 'rm -rf "$build"' EXIT
echo "Building for linux/$ARCH"
(cd "$ROOT/server" && CGO_ENABLED=0 GOOS=linux GOARCH=$ARCH go build -trimpath -ldflags "-s -w" -o "$build/shinar" ./cmd/server)

echo "Copying to $HOST"
scp "${ssh_options[@]}" "$build/shinar" "deploy@$HOST:/opt/shinar/shinar.new"
ssh "${ssh_options[@]}" "deploy@$HOST" \
    'cd /opt/shinar && { [ ! -f shinar ] || cp -p shinar shinar.previous; } && chmod 755 shinar.new && mv shinar.new shinar && sudo systemctl restart shinar'

echo "Waiting for https://$HOST/healthz"
curl --fail --silent --show-error --retry 20 --retry-delay 3 --retry-all-errors "https://$HOST/healthz"
echo
