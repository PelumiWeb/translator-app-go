#!/bin/bash
#
# Runs one of the server's waitlist commands on the host, with the server's
# settings, and prints what it says (ADR 0030). Signs in as the machine's
# admin, since only root can read the settings.
#
#   deploy/waitlist.sh -waitlist-count
#   deploy/waitlist.sh -waitlist-list
#   deploy/waitlist.sh -waitlist-invite 20 > invitations.csv
#   deploy/waitlist.sh -waitlist-remove ada@example.com
#
# make waitlist-count, waitlist-list, waitlist-invite N=20, waitlist-remove EMAIL=... run it.
set -euo pipefail

HOST=${SERVER_HOST:-shinar-api.southafricanorth.cloudapp.azure.com}
ADMIN=${SERVER_ADMIN:-azureuser}
KEY=${SERVER_ADMIN_KEY_PATH:-$HOME/.ssh/shinar_admin}
[[ $# -gt 0 ]] || { echo "usage: $0 -waitlist-count | -waitlist-list | -waitlist-invite <n> | -waitlist-remove <email>" >&2; exit 2; }

# systemd-run starts it as the service's own user with the service's
# settings, as systemd starts the server, and --pipe hands back its output.
# printf %q quotes each argument for the remote shell.
ssh -o BatchMode=yes -i "$KEY" "$ADMIN@$HOST" \
    "sudo systemd-run --quiet --wait --pipe --collect --uid=shinar --gid=shinar \
        -p EnvironmentFile=/etc/shinar/env -p WorkingDirectory=/var/lib/shinar \
        /opt/shinar/shinar $(printf '%q ' "$@")"
