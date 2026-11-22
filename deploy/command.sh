#!/bin/bash
#
# Runs one of the server's one-off commands on the host, with the server's
# settings, and prints what it says: the waitlist's (ADR 0030), and those for
# reports and accounts (ADR 0035). Signs in as the machine's admin, since
# only root can read the settings.
#
#   deploy/command.sh -waitlist-count
#   deploy/command.sh -waitlist-invite 20 > invitations.csv
#   deploy/command.sh -reports-list
#   deploy/command.sh -report-close <id> -note "what was done"
#   deploy/command.sh -account-suspend rosa.m
#   deploy/command.sh -account-delete rosa.m [-yes]
#
# The make targets waitlist-*, reports-list, report-close, account-suspend and
# account-delete run it.
set -euo pipefail

HOST=${SERVER_HOST:-shinar-api.southafricanorth.cloudapp.azure.com}
ADMIN=${SERVER_ADMIN:-azureuser}
KEY=${SERVER_ADMIN_KEY_PATH:-$HOME/.ssh/shinar_admin}
[[ $# -gt 0 ]] || { echo "usage: $0 <one of the server's command flags>, such as -waitlist-count or -reports-list" >&2; exit 2; }

# systemd-run starts it as the service's own user with the service's
# settings, as systemd starts the server, and --pipe hands back its output.
# printf %q quotes each argument for the remote shell.
ssh -o BatchMode=yes -i "$KEY" "$ADMIN@$HOST" \
    "sudo systemd-run --quiet --wait --pipe --collect --uid=shinar --gid=shinar \
        -p EnvironmentFile=/etc/shinar/env -p WorkingDirectory=/var/lib/shinar \
        /opt/shinar/shinar $(printf '%q ' "$@")"
