#!/usr/bin/env bash
# DevMail OpenDKIM entrypoint: exports DKIM keys and tables from the database
# (via devmail-tools), starts OpenDKIM, and reloads it whenever new keys appear.
set -euo pipefail

: "${DATABASE_URL:?Set DATABASE_URL}"
: "${SECRET_KEY:?Set SECRET_KEY}"
: "${DKIM_DIR:=/etc/opendkim/keys}"

mkdir -p /run/opendkim /etc/opendkim/keys
chown -R opendkim:opendkim /run/opendkim /etc/opendkim/keys

sync_keys() {
  /usr/local/bin/devmail-tools dkim-export --dir "$DKIM_DIR"
  chown -R opendkim:opendkim "$DKIM_DIR"
}

# First sync before starting so the signer is live immediately.
sync_keys || true

/usr/sbin/opendkim -f -x /etc/opendkim/opendkim.conf -P /run/opendkim/opendkim.pid &
pid=$!
trap 'kill "$pid"' TERM INT

# Refresh keys every 30s and reload the tables when a new domain key is added
# through the DevMail UI.
while true; do
  sleep 30
  if sync_keys; then
    opendkim-reload 2>/dev/null || true
  fi
done