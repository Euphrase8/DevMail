# DevMail Postfix container.
#
# Renders its config at startup from environment variables, reads the virtual
# domain/mailbox/alias tables directly from the shared DevMail PostgreSQL
# database (engine_* tables written by the backend's sql engine adapter) and
# delivers through LMTP to the Dovecot container. SASL auth is delegated to
# Dovecot over a shared unix socket.
FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends postfix postfix-pgsql openssl \
 && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /usr/local/bin/postfix-entrypoint
RUN chmod +x /usr/local/bin/postfix-entrypoint \
 && mkdir -p /var/spool/postfix/private /var/log/postfix

EXPOSE 25 587 465
ENTRYPOINT ["/usr/local/bin/postfix-entrypoint"]