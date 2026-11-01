-- Dynamic client registration accepted a client_name and then dropped it, so the consent screen and the list of connected apps could only show a random client id. Keeping the name lets people recognise what they are allowing.
ALTER TABLE oauth_clients
    ADD COLUMN IF NOT EXISTS client_name TEXT;

-- One row per person per OAuth client they have allowed. Access tokens are stateless JWTs, so revoking means deleting this row: a token carries its grant's id and is only accepted while that row exists. Allowing the client again makes a new row with a new id, so tokens from before a revoke stay dead without comparing clocks. client_name is copied here because registered clients expire long before a grant does.
CREATE TABLE IF NOT EXISTS oauth_grants (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    user_id            UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id          TEXT        NOT NULL,
    client_name        TEXT,
    redirect_uri       TEXT,
    scopes             TEXT[]      NOT NULL DEFAULT '{}',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_authorised_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, client_id)
);

---- create above / drop below ----

DROP TABLE IF EXISTS oauth_grants;
ALTER TABLE oauth_clients DROP COLUMN IF EXISTS client_name;
