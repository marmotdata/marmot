DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('view_teams', 'manage_teams', 'manage_sso_mappings')
);

DELETE FROM permissions
WHERE name IN ('view_teams', 'manage_teams', 'manage_sso_mappings');

DROP TABLE IF EXISTS sso_team_mappings;
DROP TABLE IF EXISTS asset_owners;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;

ALTER TABLE glossary_term_owners
DROP CONSTRAINT IF EXISTS fk_glossary_term_owners_team;