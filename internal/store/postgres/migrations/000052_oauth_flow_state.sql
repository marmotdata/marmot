-- A browser login spreads over four requests: POST /oauth/register, GET
-- /oauth/authorize, POST /oauth/authorize/complete and POST /oauth/token. The
-- state tying them together lived in per-process maps, so an instance running
-- more than one replica failed as soon as two of those requests landed on
-- different replicas, with "invalid_client: The requested OAuth 2.0 Client does
-- not exist". These two tables hold that state instead. Everything in them is
-- short-lived; losing a row costs at most one retried sign-in.
--
-- oauth_clients holds only the public clients minted by dynamic client
-- registration, which is why there is no secret column.
CREATE TABLE IF NOT EXISTS oauth_clients (
    client_id      TEXT        PRIMARY KEY,
    redirect_uris  TEXT[]      NOT NULL,
    grant_types    TEXT[]      NOT NULL,
    response_types TEXT[]      NOT NULL,
    scopes         TEXT[]      NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at     TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS oauth_clients_expires_at_idx ON oauth_clients (expires_at);

-- oauth_sessions is keyed by kind so the four flows needing shared state share
-- one table: authorization codes, PKCE requests, the pending authorize request
-- a browser is signing in for, and the login handoff ticket that carries a user
-- from the SSO callback to the page that signs them in. No row holds a
-- credential: the handoff stores a user id, and the token is minted when the
-- ticket is redeemed.
CREATE TABLE IF NOT EXISTS oauth_sessions (
    kind       TEXT        NOT NULL,
    session_id TEXT        NOT NULL,
    data       JSONB       NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (kind, session_id)
);

CREATE INDEX IF NOT EXISTS oauth_sessions_expires_at_idx ON oauth_sessions (expires_at);

---- create above / drop below ----

DROP TABLE IF EXISTS oauth_sessions;
DROP TABLE IF EXISTS oauth_clients;
