CREATE TABLE agent_runs (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id    VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    run_id      VARCHAR(255) NOT NULL,
    started_at  TIMESTAMPTZ  NOT NULL,
    ended_at    TIMESTAMPTZ,
    duration_ms INTEGER,
    status      VARCHAR(20)  NOT NULL,
    model       VARCHAR(255),
    tokens_in   INTEGER      NOT NULL DEFAULT 0,
    tokens_out  INTEGER      NOT NULL DEFAULT 0,
    error       TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_id, run_id)
);

CREATE INDEX idx_agent_runs_agent_started ON agent_runs (agent_id, started_at DESC);
CREATE INDEX idx_agent_runs_status        ON agent_runs (agent_id, status);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_agent_runs_status;
DROP INDEX IF EXISTS idx_agent_runs_agent_started;
DROP TABLE IF EXISTS agent_runs;
