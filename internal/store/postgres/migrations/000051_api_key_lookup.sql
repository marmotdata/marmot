-- Authenticating an API key loaded every unexpired row and bcrypt-compared each
-- one, so the cost grew linearly with the number of keys in the instance. bcrypt
-- is deliberately slow, so a few hundred keys is enough to make every request
-- expensive, and every invalid key pays the full scan.
--
-- key_lookup is an indexed SHA-256 of the key, which finds the single row to
-- verify. bcrypt still does the verifying, so a database leak is no worse than
-- before. A fast hash is correct here and would be wrong for a password: the
-- input is 32 bytes from crypto/rand, so there is no dictionary to attack.
--
-- Keys issued before this column existed cannot be backfilled, because deriving
-- the lookup needs the key and only the bcrypt hash was kept. They keep working
-- through a fallback scan limited to those rows, which shrinks as keys rotate.
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS key_lookup TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS api_keys_lookup_uq
    ON api_keys (key_lookup) WHERE key_lookup IS NOT NULL;

ALTER TABLE service_account_api_keys
    ADD COLUMN IF NOT EXISTS key_lookup TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS service_account_api_keys_lookup_uq
    ON service_account_api_keys (key_lookup) WHERE key_lookup IS NOT NULL;

---- create above / drop below ----

DROP INDEX IF EXISTS service_account_api_keys_lookup_uq;
ALTER TABLE service_account_api_keys DROP COLUMN IF EXISTS key_lookup;
DROP INDEX IF EXISTS api_keys_lookup_uq;
ALTER TABLE api_keys DROP COLUMN IF EXISTS key_lookup;
