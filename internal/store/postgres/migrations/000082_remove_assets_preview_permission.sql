-- Table previews now run through the data query system, so the assets:preview permission no longer guards anything
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE name = 'preview_assets');

DELETE FROM permissions WHERE name = 'preview_assets';

---- create above / drop below ----

INSERT INTO permissions (name, description, resource_type, action) VALUES
('preview_assets', 'Preview sample data from table assets', 'assets', 'preview')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name = 'preview_assets'
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;
