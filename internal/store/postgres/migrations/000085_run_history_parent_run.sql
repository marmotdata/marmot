-- OpenLineage runs name the run that started them in the parent run facet. Finding the runs a parent started, when the parent's events arrive after its children's, looks them up by that id.
CREATE INDEX IF NOT EXISTS idx_run_history_parent_run
    ON run_history ((COALESCE(run_facets->'parent'->'run'->>'runId', run_facets->'parentRun'->'run'->>'runId')))
    WHERE run_facets ? 'parent' OR run_facets ? 'parentRun';

---- create above / drop below ----

DROP INDEX IF EXISTS idx_run_history_parent_run;
