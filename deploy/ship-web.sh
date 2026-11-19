#!/bin/bash
#
# Builds the website and copies it to the host over SSH as the deploy user,
# where Caddy serves it beside the API (deploy/Caddyfile). The site it
# replaces stays beside it as site.previous, to go back to by hand. ADR 0030.
#
#   SERVER_HOST=shinar-api.southafricanorth.cloudapp.azure.com \
#   SERVER_SSH_KEY_PATH=~/.ssh/shinar_deploy make web-ship
set -euo pipefail

HOST=${SERVER_HOST:?the server host name}
ROOT=$(cd "$(dirname "$0")/.." && pwd)

ssh_options=(-o BatchMode=yes)
if [[ -n ${SERVER_SSH_KEY_PATH:-} ]]; then
    ssh_options+=(-i "$SERVER_SSH_KEY_PATH")
fi

make -C "$ROOT" web-build

echo "Copying to $HOST"
# COPYFILE_DISABLE and --no-xattrs keep the Mac's own file metadata out of
# the archive, which Linux's tar would otherwise warn about, file by file.
COPYFILE_DISABLE=1 tar --no-xattrs -C "$ROOT/web/out" -czf - . |
    ssh "${ssh_options[@]}" "deploy@$HOST" '
        set -e
        cd /opt/shinar
        rm -rf site.new && mkdir site.new && tar -C site.new -xzf -
        rm -rf site.previous
        if [ -d site ]; then mv site site.previous; fi
        mv site.new site'

echo "Checking https://$HOST/"
curl --fail --silent --show-error --output /dev/null "https://$HOST/"
echo "Live"
