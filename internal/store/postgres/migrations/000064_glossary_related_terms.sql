-- Terms that sit side by side rather than one under another, such as gross
-- and net revenue, are linked here. Each link is stored once in each
-- direction so a term's related terms are a single indexed lookup whichever
-- side the link was made from.
CREATE TABLE IF NOT EXISTS glossary_term_relations (
    term_id         UUID NOT NULL REFERENCES glossary_terms(id) ON DELETE CASCADE,
    related_term_id UUID NOT NULL REFERENCES glossary_terms(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (term_id, related_term_id),
    CHECK (term_id <> related_term_id)
);

CREATE INDEX IF NOT EXISTS idx_glossary_term_relations_related ON glossary_term_relations (related_term_id);

-- Who wrote a term, so its page can say. Terms created before this column
-- existed, and every term an ingestion run brings in, have no author.
ALTER TABLE glossary_terms ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;

---- create above / drop below ----

ALTER TABLE glossary_terms DROP COLUMN IF EXISTS created_by;
DROP INDEX IF EXISTS idx_glossary_term_relations_related;
DROP TABLE IF EXISTS glossary_term_relations;
