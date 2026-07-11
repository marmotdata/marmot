CREATE TABLE service_accounts (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    active      BOOLEAN NOT NULL DEFAULT true,
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX service_accounts_name_active_uq
    ON service_accounts(name) WHERE deleted_at IS NULL;

CREATE TABLE service_account_roles (
    service_account_id UUID NOT NULL REFERENCES service_accounts(id) ON DELETE CASCADE,
    role_id            UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    PRIMARY KEY (service_account_id, role_id)
);

CREATE TABLE service_account_api_keys (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_account_id UUID NOT NULL REFERENCES service_accounts(id) ON DELETE CASCADE,
    name               VARCHAR(255) NOT NULL,
    key_hash           VARCHAR(255) NOT NULL,
    last_used_at       TIMESTAMPTZ,
    expires_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (service_account_id, name)
);

CREATE INDEX idx_sa_api_keys_hash ON service_account_api_keys(key_hash);

INSERT INTO permissions (name, description, resource_type, action) VALUES
    ('service_accounts_view',   'View service accounts',                                            'service_accounts', 'view'),
    ('service_accounts_manage', 'Create, edit, delete service accounts and their API keys',         'service_accounts', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin'
  AND p.name IN ('service_accounts_view', 'service_accounts_manage');

---- create above / drop below ----

DELETE FROM role_permissions
 WHERE permission_id IN (
    SELECT id FROM permissions
    WHERE name IN ('service_accounts_view', 'service_accounts_manage')
 );
DELETE FROM permissions
 WHERE name IN ('service_accounts_view', 'service_accounts_manage');

DROP TABLE IF EXISTS service_account_api_keys;
DROP TABLE IF EXISTS service_account_roles;
DROP INDEX IF EXISTS service_accounts_name_active_uq;
DROP TABLE IF EXISTS service_accounts;
