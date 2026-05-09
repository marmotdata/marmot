CREATE TABLE agent_tool_calls (
    run_pk      UUID         NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    ordinal     INTEGER      NOT NULL,
    tool_name   VARCHAR(255) NOT NULL,
    target_mrn  VARCHAR(255),
    started_at  TIMESTAMPTZ  NOT NULL,
    duration_ms INTEGER,
    status      VARCHAR(20)  NOT NULL,
    PRIMARY KEY (run_pk, ordinal)
);

CREATE INDEX idx_agent_tool_calls_target ON agent_tool_calls (target_mrn) WHERE target_mrn IS NOT NULL;

---- create above / drop below ----

DROP INDEX IF EXISTS idx_agent_tool_calls_target;
DROP TABLE IF EXISTS agent_tool_calls;
