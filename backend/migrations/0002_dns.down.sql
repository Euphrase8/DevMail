DROP TABLE dns_records, dns_providers;
ALTER TABLE domains DROP COLUMN dkim_selector, DROP COLUMN dkim_private_key, DROP COLUMN dkim_public_key;
