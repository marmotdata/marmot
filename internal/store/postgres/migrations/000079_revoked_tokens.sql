-- Tokens signed out before they expire. Marmot's JWTs are checked on their
-- signature, so without this a copy taken before sign-out keeps working until
-- exp. Keyed by a hash of the token's jti, never the token; kept until exp, and
-- each insert prunes what has passed it.
CREATE TABLE IF NOT EXISTS revoked_tokens (
    token_key    BYTEA PRIMARY KEY,
    retain_until TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_revoked_tokens_retain_until ON revoked_tokens (retain_until);

---- create above / drop below ----

DROP TABLE IF EXISTS revoked_tokens;
