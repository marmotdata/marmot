-- Names carry a dgu_ prefix: permissions.name is unique, and an upstream
-- migration adding the same name later would fail on databases that already
-- have this row. Checks use resource_type and action, which are not unique.
INSERT INTO permissions (name, description, resource_type, action) VALUES
('dgu_view_domains', 'View domains and domain membership', 'domains', 'view'),
('dgu_manage_domains', 'Create, change, move and delete domains, and assign entities to them', 'domains', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT (SELECT id FROM roles WHERE name = 'admin'), id
  FROM permissions
 WHERE name IN ('dgu_view_domains', 'dgu_manage_domains');

INSERT INTO role_permissions (role_id, permission_id)
SELECT (SELECT id FROM roles WHERE name = 'user'), id
  FROM permissions
 WHERE name = 'dgu_view_domains';

---- create above / drop below ----

DELETE FROM role_permissions
 WHERE permission_id IN (SELECT id FROM permissions WHERE name IN ('dgu_view_domains', 'dgu_manage_domains'));
DELETE FROM permissions WHERE name IN ('dgu_view_domains', 'dgu_manage_domains');
