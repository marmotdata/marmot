CREATE TABLE user_totp (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret_ciphertext TEXT,
    pending_ciphertext TEXT,
    pending_until TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    last_step BIGINT NOT NULL DEFAULT -1,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    failure_window TIMESTAMPTZ,
    lockout_level INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ
);

CREATE TABLE user_totp_recovery (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES user_totp(user_id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used_at TIMESTAMPTZ
);

CREATE TABLE user_auth_challenges (
    token_hash BYTEA PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('totp', 'password_change', 'enrollment')),
    password_fingerprint BYTEA NOT NULL,
    session_epoch TIMESTAMPTZ,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX user_auth_challenges_user ON user_auth_challenges(user_id);
CREATE INDEX user_auth_challenges_expiry ON user_auth_challenges(expires_at);

---- create above / drop below ----
DROP TABLE user_auth_challenges;
DROP TABLE user_totp_recovery;
DROP TABLE user_totp;
