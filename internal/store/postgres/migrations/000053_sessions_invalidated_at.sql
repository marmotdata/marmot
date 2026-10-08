ALTER TABLE users ADD COLUMN sessions_invalidated_at TIMESTAMP WITH TIME ZONE;

COMMENT ON COLUMN users.sessions_invalidated_at IS 'Tokens issued before this moment are rejected; set by sign-out-all and password changes';

---- create above / drop below ----

ALTER TABLE users DROP COLUMN sessions_invalidated_at;
