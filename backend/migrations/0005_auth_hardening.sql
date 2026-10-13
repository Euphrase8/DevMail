ALTER TABLE users ADD COLUMN totp_pending text;
ALTER TABLE users ADD COLUMN totp_pending_until timestamptz;
ALTER TABLE users ADD COLUMN totp_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN totp_last_step bigint NOT NULL DEFAULT -1;
ALTER TABLE users ADD COLUMN recovery_hashes text[] NOT NULL DEFAULT '{}';
ALTER TABLE users ADD COLUMN last_login_at timestamptz;
CREATE TABLE auth_attempts (
 key text PRIMARY KEY, attempts integer NOT NULL DEFAULT 1,
 window_start timestamptz NOT NULL DEFAULT now(), locked_until timestamptz
);
ALTER TABLE sessions ADD COLUMN last_seen_at timestamptz NOT NULL DEFAULT now();
