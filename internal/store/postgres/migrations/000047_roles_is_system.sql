BEGIN;

ALTER TABLE roles
  ADD COLUMN is_system BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN deleted_at TIMESTAMPTZ;

UPDATE roles SET is_system = true WHERE name IN ('admin', 'user');

-- Partial unique index supports soft-delete: same name can be reused after deletion.
CREATE UNIQUE INDEX roles_name_active_uq ON roles(name) WHERE deleted_at IS NULL;

-- The old unique constraint is now covered by the partial index for active rows.
ALTER TABLE roles DROP CONSTRAINT roles_name_key;

COMMIT;

---- create above / drop below ----

BEGIN;

DROP INDEX IF EXISTS roles_name_active_uq;
ALTER TABLE roles ADD CONSTRAINT roles_name_key UNIQUE (name);
ALTER TABLE roles DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE roles DROP COLUMN IF EXISTS is_system;

COMMIT;
