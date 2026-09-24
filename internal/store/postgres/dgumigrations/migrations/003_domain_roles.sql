-- subject_id has no foreign key: it points at users, teams or service
-- accounts. Scopes are resolved from the acting principal outward, so a row
-- whose subject was deleted can never match anyone; listings mark it.
CREATE TABLE domain_role_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('user', 'team', 'service_account')),
    subject_id UUID NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('domain_admin', 'steward', 'reader')),
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (domain_id, subject_type, subject_id, role)
);
CREATE INDEX domain_role_assignments_subject_idx ON domain_role_assignments (subject_type, subject_id);

---- create above / drop below ----

DROP TABLE domain_role_assignments;
