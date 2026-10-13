CREATE TABLE routing_rules (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL,
 config jsonb NOT NULL, enabled boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE backup_targets (LIKE routing_rules INCLUDING ALL);
CREATE TABLE notification_channels (LIKE routing_rules INCLUDING ALL);
CREATE TABLE backup_runs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), target_id uuid NOT NULL REFERENCES backup_targets(id) ON DELETE CASCADE,
 status text NOT NULL, detail text NOT NULL DEFAULT '', bytes bigint NOT NULL DEFAULT 0,
 created_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz
);
CREATE TABLE notification_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), channel_id uuid NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
 status text NOT NULL, detail text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
