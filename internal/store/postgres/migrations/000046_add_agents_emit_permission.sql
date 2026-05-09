-- Permission for SDK service tokens to write agent run telemetry.
INSERT INTO permissions (name, description, resource_type, action) VALUES
('emit_agent_runs', 'Record agent run telemetry (invocations, tool calls)', 'agents', 'emit');

-- Grant to admin only by default; service tokens are issued explicitly per agent.
INSERT INTO role_permissions (role_id, permission_id)
SELECT
    (SELECT id FROM roles WHERE name = 'admin'),
    id
FROM permissions
WHERE name = 'emit_agent_runs';

---- create above / drop below ----

DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE name = 'emit_agent_runs');
DELETE FROM permissions WHERE name = 'emit_agent_runs';
