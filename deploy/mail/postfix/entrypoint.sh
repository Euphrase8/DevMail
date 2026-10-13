#!/usr/bin/env bash
# DevMail Postfix entrypoint: renders configuration from environment
# variables and starts Postfix in the foreground.
set -euo pipefail

# --- configuration -----------------------------------------------------------
: "${MAIL_HOSTNAME:=mail.localhost}"
: "${MAIL_DOMAIN:=localhost}"
: "${POSTGRES_HOST:=postgres}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_DB:=devmail}"
: "${POSTGRES_USER:=postgres}"
: "${POSTGRES_PASSWORD:?Set POSTGRES_PASSWORD}"
: "${POSTFIX_MYNETWORKS:=127.0.0.0/8}"
: "${MAIL_TLS_CERT_FILE:=/etc/ssl/private/mail.pem}"
: "${MAIL_TLS_KEY_FILE:=/etc/ssl/private/mail.key}"

mkdir -p /etc/postfix/sql /var/log/postfix /var/spool/postfix/private

# --- TLS ---------------------------------------------------------------------
# A self-signed certificate is generated when none is mounted so the services
# start; mount real certificates to replace it (certs are shared with Dovecot
# through the mail-certs volume).
if [ ! -f "$MAIL_TLS_CERT_FILE" ] || [ ! -f "$MAIL_TLS_KEY_FILE" ]; then
  echo "postfix: generating self-signed certificate for $MAIL_HOSTNAME"
  openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
    -keyout "$MAIL_TLS_KEY_FILE" -out "$MAIL_TLS_CERT_FILE" \
    -subj "/CN=$MAIL_HOSTNAME" >/dev/null 2>&1
fi
chmod 600 "$MAIL_TLS_KEY_FILE"

# --- PostgreSQL lookup maps --------------------------------------------------
cat > /etc/postfix/sql/virtual_domains.cf <<EOF
hosts = $POSTGRES_HOST:$POSTGRES_PORT
user = $POSTGRES_USER
password = $POSTGRES_PASSWORD
dbname = $POSTGRES_DB
query = SELECT domain FROM engine_domains WHERE domain = '%s' AND enabled = TRUE AND deleted_at IS NULL
EOF

cat > /etc/postfix/sql/virtual_mailboxes.cf <<EOF
hosts = $POSTGRES_HOST:$POSTGRES_PORT
user = $POSTGRES_USER
password = $POSTGRES_PASSWORD
dbname = $POSTGRES_DB
query = SELECT email FROM engine_mailboxes WHERE email = '%s' AND enabled = TRUE AND deleted_at IS NULL
EOF

cat > /etc/postfix/sql/virtual_aliases.cf <<EOF
hosts = $POSTGRES_HOST:$POSTGRES_PORT
user = $POSTGRES_USER
password = $POSTGRES_PASSWORD
dbname = $POSTGRES_DB
query = SELECT destination FROM engine_aliases WHERE source = '%s' AND enabled = TRUE AND deleted_at IS NULL
EOF
chmod 600 /etc/postfix/sql/*.cf

# --- main.cf -----------------------------------------------------------------
cat > /etc/postfix/main.cf <<EOF
compatibility_level = 2

myhostname = $MAIL_HOSTNAME
mydomain = $MAIL_DOMAIN
myorigin = \$myhostname

inet_interfaces = all
inet_protocols = ipv4

# Only localhost; all mail domains are virtual and come from the database.
mydestination = localhost.\$mydomain, localhost
virtual_mailbox_domains = pgsql:/etc/postfix/sql/virtual_domains.cf
virtual_mailbox_maps   = pgsql:/etc/postfix/sql/virtual_mailboxes.cf
virtual_alias_maps     = pgsql:/etc/postfix/sql/virtual_aliases.cf
virtual_transport      = lmtp:unix:private/dovecot-lmtp
local_transport        = error:local delivery is not configured

mynetworks = $POSTFIX_MYNETWORKS
smtpd_banner = \$myhostname ESMTP

smtpd_relay_restrictions   = permit_mynetworks, permit_sasl_authenticated, defer_unauth_destination
smtpd_recipient_restrictions = permit_mynetworks, permit_sasl_authenticated, reject_unauth_destination
smtpd_helo_required        = yes
disable_vrfy_command       = yes

# SASL against Dovecot over the shared unix socket.
smtpd_sasl_type = dovecot
smtpd_sasl_path = private/auth
smtpd_sasl_auth_enable = no
broken_sasl_auth_clients  = yes

# TLS.
smtpd_tls_cert_file = $MAIL_TLS_CERT_FILE
smtpd_tls_key_file  = $MAIL_TLS_KEY_FILE
smtpd_tls_security_level = may
smtpd_tls_loglevel  = 1
tls_random_source   = dev:/dev/urandom

# OpenDKIM milter.
smtpd_milters            = inet:opendkim:8891
non_smtpd_milters        = \$smtpd_milters
milter_default_action    = accept

# Limits.
mailbox_size_limit = 0
message_size_limit = 25600000
mailbox_command    = /nonexistent
mail_spool_directory = /var/vmail

# Log to a file; no syslog required in the container.
maillog_file = /var/log/postfix/mail.log
EOF

# --- master.cf ---------------------------------------------------------------
cat > /etc/postfix/master.cf <<'EOF'
# The standard Postfix master.cf with hardened inbound and submission entries.
smtp      inet  n       -       y       -       -       smtpd
  -o smtpd_tls_security_level=may
pickup    unix  n       -       y       60      1       pickup
cleanup   unix  n       -       y       -       0       cleanup
qmgr      unix  n       -       n       300     1       qmgr
tlsmgr    unix  -       -       y       1000?   1       tlsmgr
rewrite   unix  -       -       y       -       -       trivial-rewrite
bounce    unix  -       -       y       -       0       bounce
defer     unix  -       -       y       -       0       bounce
trace     unix  -       -       y       -       0       bounce
verify    unix  -       -       y       -       1       verify
flush     unix  n       -       y       1000?   0       flush
proxymap  unix  -       -       n       -       -       proxymap
proxywrite unix -       -       n       -       1       proxymap
smtp      unix  -       -       y       -       -       smtp
relay     unix  -       -       y       -       -       smtp
  -o syslog_name=postfix/$service_name
showq     unix  n       -       y       -       -       showq
error     unix  -       -       y       -       -       error
retry     unix  -       -       y       -       -       error
discard   unix  -       -       y       -       -       discard
local     unix  -       n       n       -       -       local
virtual   unix  -       n       n       -       -       virtual
lmtp      unix  -       -       y       -       -       lmtp
anvil     unix  -       -       y       -       1       anvil
scache    unix  -       -       y       -       1       scache
postlog   unix-dgram n  -       n       -       1       postlogd

# Authenticated submission (587) with mandatory TLS.
submission inet n       -       y       -       -       smtpd
  -o syslog_name=postfix/submission
  -o smtpd_tls_security_level=encrypt
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_sasl_type=dovecot
  -o smtpd_sasl_path=private/auth
  -o smtpd_relay_restrictions=permit_sasl_authenticated,reject
  -o smtpd_client_restrictions=permit_sasl_authenticated,reject
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject
  -o milter_macro_daemon_name=ORIGINATING

# SMTPS (465): implicit TLS, authenticated.
smtps inet n       -       y       -       -       smtpd
  -o syslog_name=postfix/smtps
  -o smtpd_tls_wrappermode=yes
  -o smtpd_tls_security_level=encrypt
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_sasl_type=dovecot
  -o smtpd_sasl_path=private/auth
  -o smtpd_relay_restrictions=permit_sasl_authenticated,reject
  -o smtpd_client_restrictions=permit_sasl_authenticated,reject
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject
  -o milter_macro_daemon_name=ORIGINATING
EOF

# Make the shared socket directory usable by both containers (dovecot creates
# the sockets; postfix connects through them).
chmod 1777 /var/spool/postfix/private

exec postfix start-fg