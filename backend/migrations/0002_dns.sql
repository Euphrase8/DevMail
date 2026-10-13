-- 0002_dns.sql — DNS record planning, provider connections, and DKIM keys.

-- DKIM material stored per domain. The private key is encrypted at rest with
-- the application SECRET_KEY; the public key is published in a TXT record.
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dkim_selector    text NOT NULL DEFAULT '';
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dkim_private_key text NOT NULL DEFAULT '';
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dkim_public_key  text NOT NULL DEFAULT '';

-- DNS providers hold encrypted credentials. The token column is never
-- returned by the API; only a boolean "hasToken" flag is exposed.
CREATE TABLE IF NOT EXISTS dns_providers (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name           text NOT NULL,
    kind           text NOT NULL DEFAULT 'manual'
                   CHECK (kind IN ('manual','cloudflare','route53')),
    zone           text NOT NULL DEFAULT '',
    token_enc      text NOT NULL DEFAULT '',
    status         text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','connected','error','disabled')),
    last_verified_at timestamptz,
    last_verify_error text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);
CREATE INDEX IF NOT EXISTS idx_dns_providers_kind ON dns_providers(kind);

-- Planned/applied DNS records for a managed domain. A (domain,type,name)
-- tuple is unique so re-planning updates rather than duplicates.
CREATE TABLE IF NOT EXISTS dns_records (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id   uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    provider_id uuid REFERENCES dns_providers(id) ON DELETE SET NULL,
    type        text NOT NULL,
    name        text NOT NULL,
    value       text NOT NULL DEFAULT '',
    ttl         integer NOT NULL DEFAULT 3600,
    priority    integer NOT NULL DEFAULT 0,
    purpose     text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'planned'
                CHECK (status IN ('planned','published','manual','error')),
    detail      text NOT NULL DEFAULT '',
    applied_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (domain_id, type, name)
);
CREATE INDEX IF NOT EXISTS idx_dns_records_domain ON dns_records(domain_id);
