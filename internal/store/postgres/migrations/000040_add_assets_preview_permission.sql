-- Add preview permission for assets
INSERT INTO permissions (name, description, resource_type, action) VALUES
('preview_assets', 'Preview sample data from table assets', 'assets', 'preview');

-- Grant preview permission to admin role only (least privilege by default)
INSERT INTO role_permissions (role_id, permission_id)
SELECT
    (SELECT id FROM roles WHERE name = 'admin'),
    id
FROM permissions
WHERE name = 'preview_assets';

---- create above / drop below ----

DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE name = 'preview_assets');
DELETE FROM permissions WHERE name = 'preview_assets';
