DELETE FROM role_permissions 
WHERE role_id = (SELECT id FROM roles WHERE name = 'user') 
AND permission_id = (SELECT id FROM permissions WHERE name = 'view_metrics');

DELETE FROM role_permissions 
WHERE role_id = (SELECT id FROM roles WHERE name = 'admin') 
AND permission_id = (SELECT id FROM permissions WHERE name = 'view_metrics');

DELETE FROM permissions WHERE name = 'view_metrics';
