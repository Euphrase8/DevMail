# DevMail Dovecot container.
#
# Authenticates IMAP logins and accepts LMTP delivery from Postfix against the
# engine_* tables in the shared DevMail PostgreSQL database. Passwords are
# Argon2id hashes ({ARGON2ID}$argon2id$...) produced by the backend, which
# Debian's dovecot (built with libsodium) verifies directly.
FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends dovecot-imapd dovecot-lmtpd dovecot-pgsql openssl \
 && rm -rf /var/lib/apt/lists/*

# vmail owns the shared mail storage volume. mail-sockets must stay writable by
# dovecot for the LMTP/auth sockets Postfix connects to.
RUN groupadd -g 5000 vmail && useradd -u 5000 -g 5000 -s /usr/sbin/nologin -M vmail \
 && mkdir -p /var/mail/vmail /var/spool/postfix/private \
 && chown -R vmail:vmail /var/mail/vmail

RUN rm -rf /etc/dovecot/conf.d /etc/dovecot/dovecot.conf /etc/dovecot/dovecot-sql.conf.ext
COPY dovecot.conf /etc/dovecot/dovecot.conf
COPY conf.d/ /etc/dovecot/conf.d/
COPY entrypoint.sh /usr/local/bin/dovecot-entrypoint
RUN chmod +x /usr/local/bin/dovecot-entrypoint \
 && chown -R root:dovecot /etc/dovecot

EXPOSE 993
ENTRYPOINT ["/usr/local/bin/dovecot-entrypoint"]