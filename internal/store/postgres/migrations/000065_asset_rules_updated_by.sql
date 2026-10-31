-- Record who last changed an asset rule, so its page can name them. A rule
-- nobody has edited since it was created was last changed by its author.
ALTER TABLE asset_rules ADD COLUMN IF NOT EXISTS updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

UPDATE asset_rules SET updated_by = created_by WHERE updated_by IS NULL AND created_by IS NOT NULL;

---- create above / drop below ----

ALTER TABLE asset_rules DROP COLUMN IF EXISTS updated_by;
