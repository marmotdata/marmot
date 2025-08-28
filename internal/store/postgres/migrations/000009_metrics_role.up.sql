-- Add metrics permission
INSERT INTO permissions (name, description, resource_type, action) VALUES 
('view_metrics', 'View system metrics and analytics', 'metrics', 'view');

-- Assign metrics permission to admin role
INSERT INTO role_permissions (role_id, permission_id)
SELECT 
    (SELECT id FROM roles WHERE name = 'admin'),
    id
FROM permissions 
WHERE name = 'view_metrics';

-- Assign metrics permission to user role
INSERT INTO role_permissions (role_id, permission_id)
SELECT 
    (SELECT id FROM roles WHERE name = 'user'),
    id
FROM permissions 
WHERE name = 'view_metrics';
