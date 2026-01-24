ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture TEXT;

---- create above / drop below ----

ALTER TABLE users DROP COLUMN IF EXISTS profile_picture;
