CREATE TABLE IF NOT EXISTS global_documentation (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    content TEXT NOT NULL,
    source VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_global_source UNIQUE (source)
);

CREATE TABLE IF NOT EXISTS documentation (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    mrn VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    source VARCHAR(255) NOT NULL,
    global_docs TEXT[],
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_mrn_source UNIQUE (mrn, source)
);

CREATE INDEX IF NOT EXISTS idx_documentation_mrn ON documentation(mrn);
CREATE INDEX IF NOT EXISTS idx_documentation_source ON documentation(source);
CREATE INDEX IF NOT EXISTS idx_global_documentation_source ON global_documentation(source);

