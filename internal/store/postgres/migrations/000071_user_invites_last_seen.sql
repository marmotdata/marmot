-- When a member last made an authenticated request. Written at most every few minutes per member, so it answers "is this person still around" rather than auditing each request.
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMP WITH TIME ZONE;

-- An invitation is a pending account plus a one-time link. The account exists from the start so roles and teams can be set before the person arrives, and it stays inactive with no password until the link is used. Only a hash of the link's token is kept, so reading this table never yields a working link.
CREATE TABLE IF NOT EXISTS user_invites (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    email      VARCHAR(255) NOT NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_invites_email ON user_invites (lower(email));

---- create above / drop below ----

DROP TABLE IF EXISTS user_invites;
ALTER TABLE users DROP COLUMN IF EXISTS last_seen_at;
