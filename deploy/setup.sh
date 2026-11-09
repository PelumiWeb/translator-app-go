#!/bin/bash
#
# Prepares a fresh Ubuntu 24.04 machine to run the Shinar server: Caddy in
# front for HTTPS, the server as a systemd service, its folders, the speech
# model, and a "deploy" user that ships new builds (deploy/ship.sh). Run it
# once, with sudo, from a copy of this folder on the machine:
#
#   sudo bash deploy/setup.sh <host name> "<deploy public key>"
#
# Running it again is safe. The server's settings, /etc/shinar/env, are
# copied on their own; ADR 0023 has the steps.
set -euo pipefail

HOST=${1:?the host name, such as shinar-api.southafricanorth.cloudapp.azure.com}
DEPLOY_KEY=${2:?the public key that ships new builds}
HERE=$(cd "$(dirname "$0")" && pwd)

# The model the phones download, checked against the copy the app was built with.
MODEL=ggml-base-q5_1.bin
MODEL_URL=https://huggingface.co/ggerganov/whisper.cpp/resolve/main/$MODEL
MODEL_SHA256=422f1ae452ade6f30a004d7e5c6a43195e4433bc370bf23fac9cc591f01a8898

export DEBIAN_FRONTEND=noninteractive

echo "== Packages"
apt-get update -q
apt-get install -y -q curl gpg debian-keyring debian-archive-keyring apt-transport-https postgresql-client ufw
# Caddy from its own repository, as Caddy's documentation advises: Ubuntu's
# copy is years behind.
if [[ ! -f /usr/share/keyrings/caddy-stable-archive-keyring.gpg ]]; then
    curl -1sLf https://dl.cloudsmith.io/public/caddy/stable/gpg.key | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt > /etc/apt/sources.list.d/caddy-stable.list
    apt-get update -q
fi
apt-get install -y -q caddy

echo "== Swap"
# 1 GB of memory and no swap: a little swap keeps a spike from killing the server.
if [[ ! -f /swapfile ]]; then
    fallocate -l 1G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

echo "== Users and folders"
id shinar >/dev/null 2>&1 || useradd --system --home-dir /var/lib/shinar --shell /usr/sbin/nologin shinar
id deploy >/dev/null 2>&1 || useradd --create-home --shell /bin/bash deploy
install -d -m 700 -o deploy -g deploy /home/deploy/.ssh
echo "$DEPLOY_KEY" > /home/deploy/.ssh/authorized_keys
chown deploy:deploy /home/deploy/.ssh/authorized_keys
chmod 600 /home/deploy/.ssh/authorized_keys
# The deploy user may put a new program in place and restart it, nothing more.
install -d -m 755 -o deploy -g deploy /opt/shinar
echo 'deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart shinar' > /etc/sudoers.d/shinar-deploy
chmod 440 /etc/sudoers.d/shinar-deploy
visudo -cf /etc/sudoers.d/shinar-deploy
for dir in audio voices messages models; do
    install -d -m 750 -o shinar -g shinar "/var/lib/shinar/$dir"
done
install -d -m 750 /etc/shinar
[[ -f /etc/shinar/env ]] || install -m 600 /dev/null /etc/shinar/env

echo "== Speech model"
model=/var/lib/shinar/models/$MODEL
if ! echo "$MODEL_SHA256  $model" | sha256sum --check --status 2>/dev/null; then
    curl -fsSL -o "$model.part" "$MODEL_URL"
    echo "$MODEL_SHA256  $model.part" | sha256sum --check --status
    mv "$model.part" "$model"
    chown shinar:shinar "$model"
fi

echo "== Service and HTTPS"
install -m 644 "$HERE/shinar.service" /etc/systemd/system/shinar.service
sed "s/{{HOST}}/$HOST/" "$HERE/Caddyfile" > /etc/caddy/Caddyfile
systemctl daemon-reload
# Enabled now, started by the first deploy, once there is a program to run.
systemctl enable shinar
systemctl reload caddy || systemctl restart caddy

echo "== Firewall"
ufw allow OpenSSH
ufw allow http
ufw allow https
ufw --force enable

echo "Ready. Next: the settings in /etc/shinar/env, then the first deploy (ADR 0023)."
