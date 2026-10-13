#!/usr/bin/env bash
# DevMail Dovecot entrypoint: renders the SQL auth settings from environment
# variables, ensures TLS material exists, and starts Dovecot in the foreground.
set -euo pipefail

: "${MAIL_HOSTNAME:=mail.localhost}"
: "${POSTGRES_HOST:=postgres}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_DB:=devmail}"
: "${POSTGRES_USER:=postgres}"
: "${POSTGRES_PASSWORD:?Set POSTGRES_PASSWORD}"
: "${MAIL_TLS_CERT_FILE:=/etc/ssl/private/mail.pem}"
: "${MAIL_TLS_KEY_FILE:=/etc/ssl/private/mail.key}"

mkdir -p /var/spool/postfix/private
chmod 1777 /var/spool/postfix/private

# Generate a self-signed certificate when none is shared yet (postfix writes
# the same paths into the mail-certs volume; either container can generate it).
if [ ! -f "$MAIL_TLS_CERT_FILE" ] || [ ! -f "$MAIL_TLS_KEY_FILE" ]; then
  echo "dovecot: generating self-signed certificate for $MAIL_HOSTNAME"
  mkdir -p /etc/ssl/private
  openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
    -keyout "$MAIL_TLS_KEY_FILE" -out "$MAIL_TLS_CERT_FILE" \
    -subj "/CN=$MAIL_HOSTNAME" >/dev/null 2>&1
fi

cat > /etc/dovecot/dovecot-sql.conf.ext <<EOF
driver = pgsql
connect = host=$POSTGRES_HOST port=$POSTGRES_PORT dbname=$POSTGRES_DB user=$POSTGRES_USER password=$POSTGRES_PASSWORD

# Stored hashes are self-describing ({ARGON2ID}\$argon2id\$...) so no
# default_pass_scheme is needed.
password_query = SELECT email AS user, password_hash AS password \
  FROM engine_mailboxes WHERE email = '%u' AND enabled = TRUE AND deleted_at IS NULL

user_query = SELECT email AS user, \
  '/var/mail/vmail/' || domain || '/' || local AS home, \
  5000 AS uid, 5000 AS gid, \
  'maildir:/var/mail/vmail/' || domain || '/' || local AS mail \
  FROM engine_mailboxes WHERE email = '%u' AND enabled = TRUE AND deleted_at IS NULL

iterate_query = SELECT email AS user FROM engine_mailboxes WHERE enabled = TRUE AND deleted_at IS NULL
EOF
chmod 640 /etc/dovecot/dovecot-sql.conf.ext

exec dovecot -F