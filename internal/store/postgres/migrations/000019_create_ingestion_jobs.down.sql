DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('view_ingestion', 'manage_ingestion')
);

DELETE FROM permissions WHERE name IN ('view_ingestion', 'manage_ingestion');

DROP TABLE IF EXISTS ingestion_job_runs;
DROP TABLE IF EXISTS ingestion_schedules;
