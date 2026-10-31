-- A group mapping can now give a role as well as, or instead of, a team, so the team becomes optional and the role is a reference that disappears with the role rather than blocking its deletion.
ALTER TABLE sso_team_mappings ALTER COLUMN team_id DROP NOT NULL;
ALTER TABLE sso_team_mappings ADD COLUMN IF NOT EXISTS role_id UUID REFERENCES roles(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_sso_team_mappings_role_id ON sso_team_mappings (role_id) WHERE role_id IS NOT NULL;

---- create above / drop below ----

DROP INDEX IF EXISTS idx_sso_team_mappings_role_id;
DELETE FROM sso_team_mappings WHERE team_id IS NULL;
ALTER TABLE sso_team_mappings DROP COLUMN IF EXISTS role_id;
ALTER TABLE sso_team_mappings ALTER COLUMN team_id SET NOT NULL;
