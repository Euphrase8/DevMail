-- Keep forward-only evolution of previously deployed migrations.
CREATE INDEX idx_jobs_resource ON jobs(kind,status);
CREATE INDEX idx_audit_action ON audit_log(action,created_at DESC);
CREATE INDEX idx_auth_attempt_window ON auth_attempts(window_start);
