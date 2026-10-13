DROP TRIGGER mailbox_outbox ON mailboxes;
DROP TRIGGER alias_outbox ON aliases;
DROP FUNCTION provision_record_outbox();
DROP TABLE engine_aliases, engine_mailboxes, engine_domains;
ALTER TABLE mailboxes DROP COLUMN password_hash, DROP COLUMN last_provisioned_at, DROP COLUMN provisioning_error;
ALTER TABLE aliases DROP COLUMN last_provisioned_at;
ALTER TABLE domains DROP COLUMN last_provisioned_at, DROP COLUMN provisioning_error;
DELETE FROM settings WHERE key='provisioning_enabled';
