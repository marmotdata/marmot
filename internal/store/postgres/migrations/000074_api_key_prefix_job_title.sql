-- A key is only ever shown once, so without a visible fragment nobody can tell which key in a list is the one sitting in a config file. key_prefix keeps the scheme prefix plus the first few random characters, which identifies the key without weakening it. Keys issued before this column cannot be backfilled because only the bcrypt hash was kept, so theirs stays NULL until they are rotated.
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS key_prefix TEXT;

ALTER TABLE service_account_api_keys
    ADD COLUMN IF NOT EXISTS key_prefix TEXT;

-- Job title is something a person sets about themselves on their account page rather than something an identity provider supplies, so it is a plain nullable column.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS job_title VARCHAR(255);

---- create above / drop below ----

ALTER TABLE users DROP COLUMN IF EXISTS job_title;
ALTER TABLE service_account_api_keys DROP COLUMN IF EXISTS key_prefix;
ALTER TABLE api_keys DROP COLUMN IF EXISTS key_prefix;
