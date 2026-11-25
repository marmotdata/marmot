-- Each key generation gains an approval key: an ECDSA P-256 key that signs only approvals of actions, never queries, so the query signing key can never produce an approval and the approval key can later move behind a service of its own. Like the other keys it is either encrypted here or named by KMS URI. A generation made before this has none until Marmot starts and adds one, or, for KMS-held keys, until a rotation names one.
ALTER TABLE plugin_serve_keys
    ADD COLUMN IF NOT EXISTS approval_public_key TEXT,
    ADD COLUMN IF NOT EXISTS approval_key TEXT,
    ADD COLUMN IF NOT EXISTS approval_key_uri TEXT;

ALTER TABLE plugin_serve_keys ADD CONSTRAINT plugin_serve_keys_approval_key_one CHECK (
    (approval_public_key IS NULL AND approval_key IS NULL AND approval_key_uri IS NULL)
    OR (approval_public_key IS NOT NULL AND ((approval_key IS NULL) <> (approval_key_uri IS NULL))));

---- create above / drop below ----

ALTER TABLE plugin_serve_keys DROP CONSTRAINT IF EXISTS plugin_serve_keys_approval_key_one;
ALTER TABLE plugin_serve_keys
    DROP COLUMN IF EXISTS approval_public_key,
    DROP COLUMN IF EXISTS approval_key,
    DROP COLUMN IF EXISTS approval_key_uri;
