DROP TABLE auth_attempts;
ALTER TABLE sessions DROP COLUMN last_seen_at;
ALTER TABLE users DROP COLUMN totp_pending, DROP COLUMN totp_pending_until, DROP COLUMN totp_enabled, DROP COLUMN totp_last_step, DROP COLUMN recovery_hashes, DROP COLUMN last_login_at;
