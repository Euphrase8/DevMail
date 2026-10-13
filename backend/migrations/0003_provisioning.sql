-- 0003_provisioning.sql — mail-engine provisioning state.
-- The SQL adapter mirrors a Dovecot/Postfix-style virtual users schema so a real
-- mail server can consume these rows directly. When no engine is configured the
-- adapter is a no-op and records remain "unmanaged".

CREATE TABLE IF NOT EXISTS engine_domains (
    domain     text PRIMARY KEY,
    enabled    boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TABLE IF NOT EXISTS engine_mailboxes (
    email         text PRIMARY KEY,
    domain        text NOT NULL,
    local         text NOT NULL,
    password_hash text NOT NULL DEFAULT '',
    quota_bytes   bigint NOT NULL DEFAULT 0,
    enabled       boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);
CREATE INDEX IF NOT EXISTS idx_engine_mailboxes_domain ON engine_mailboxes(domain);

CREATE TABLE IF NOT EXISTS engine_aliases (
    source      text PRIMARY KEY,
    destination text NOT NULL,
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);

-- Provisioning bookkeeping on the managed records.
ALTER TABLE mailboxes ADD COLUMN IF NOT EXISTS password_hash text NOT NULL DEFAULT '';
ALTER TABLE mailboxes ADD COLUMN IF NOT EXISTS last_provisioned_at timestamptz;
ALTER TABLE mailboxes ADD COLUMN IF NOT EXISTS provisioning_error text NOT NULL DEFAULT '';
ALTER TABLE aliases ADD COLUMN IF NOT EXISTS last_provisioned_at timestamptz;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS last_provisioned_at timestamptz;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS provisioning_error text NOT NULL DEFAULT '';

-- Queue writes are atomic with mailbox/alias mutations; IDs identify idempotent
-- desired-state operations and passwords never enter the queue.
CREATE FUNCTION provision_record_outbox() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE k text; payload jsonb;
BEGIN
 IF COALESCE((SELECT value::text = 'true' FROM settings WHERE key='provisioning_enabled'),false) = false THEN RETURN NEW; END IF;
 IF TG_TABLE_NAME='mailboxes' THEN
  IF NEW.deleted_at IS NOT NULL AND (TG_OP='INSERT' OR OLD.deleted_at IS NULL) THEN k := 'mailbox.delete';
  ELSIF TG_OP='INSERT' OR NEW.password_hash IS DISTINCT FROM OLD.password_hash OR NEW.quota_bytes IS DISTINCT FROM OLD.quota_bytes THEN k := 'mailbox.create';
  ELSE RETURN NEW; END IF;
  payload := jsonb_build_object('mailboxId',NEW.id::text);
  NEW.engine_status := 'provisioning';
 ELSE
  IF NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
   k := 'alias.delete'; payload := jsonb_build_object('source',NEW.source || '@' || (SELECT name FROM domains WHERE id=NEW.domain_id));
  ELSIF TG_OP='INSERT' THEN k := 'alias.provision'; payload := jsonb_build_object('aliasId',NEW.id::text);
  ELSE RETURN NEW; END IF;
 END IF;
 INSERT INTO jobs(kind,payload) VALUES(k,payload);
 RETURN NEW;
END $$;
CREATE TRIGGER mailbox_outbox BEFORE INSERT OR UPDATE ON mailboxes FOR EACH ROW EXECUTE FUNCTION provision_record_outbox();
CREATE TRIGGER alias_outbox BEFORE INSERT OR UPDATE ON aliases FOR EACH ROW EXECUTE FUNCTION provision_record_outbox();
