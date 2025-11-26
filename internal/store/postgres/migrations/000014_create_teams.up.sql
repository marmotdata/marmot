CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_via_sso BOOLEAN NOT NULL DEFAULT FALSE,
    sso_provider VARCHAR(50),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_teams_created_via_sso ON teams(created_via_sso);
CREATE INDEX IF NOT EXISTS idx_teams_sso_provider ON teams(sso_provider);
CREATE INDEX IF NOT EXISTS idx_teams_updated_at ON teams(updated_at);

CREATE TABLE IF NOT EXISTS team_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('owner', 'member')),
    source VARCHAR(20) NOT NULL CHECK (source IN ('manual', 'sso')),
    sso_provider VARCHAR(50),
    joined_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_team_members_team_id ON team_members(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id);
CREATE INDEX IF NOT EXISTS idx_team_members_source ON team_members(source);

CREATE TABLE IF NOT EXISTS asset_owners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK ((user_id IS NOT NULL AND team_id IS NULL) OR (user_id IS NULL AND team_id IS NOT NULL)),
    UNIQUE (asset_id, user_id),
    UNIQUE (asset_id, team_id)
);

CREATE INDEX IF NOT EXISTS idx_asset_owners_asset ON asset_owners(asset_id);
CREATE INDEX IF NOT EXISTS idx_asset_owners_user ON asset_owners(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_asset_owners_team ON asset_owners(team_id) WHERE team_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS sso_team_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL,
    sso_group_name VARCHAR(255) NOT NULL,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    member_role VARCHAR(20) NOT NULL CHECK (member_role IN ('owner', 'member')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (provider, sso_group_name)
);

CREATE INDEX IF NOT EXISTS idx_sso_team_mappings_provider ON sso_team_mappings(provider);
CREATE INDEX IF NOT EXISTS idx_sso_team_mappings_team_id ON sso_team_mappings(team_id);

INSERT INTO permissions (name, description, resource_type, action) VALUES
('view_teams', 'View teams', 'teams', 'view'),
('manage_teams', 'Create/update/delete teams', 'teams', 'manage'),
('manage_sso_mappings', 'Manage SSO team mappings', 'sso', 'manage')
ON CONFLICT (resource_type, action) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT
    (SELECT id FROM roles WHERE name = 'admin'),
    id
FROM permissions
WHERE name IN ('view_teams', 'manage_teams', 'manage_sso_mappings')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT
    (SELECT id FROM roles WHERE name = 'user'),
    id
FROM permissions
WHERE name = 'view_teams'
ON CONFLICT DO NOTHING;

ALTER TABLE glossary_term_owners
ADD CONSTRAINT fk_glossary_term_owners_team
FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;