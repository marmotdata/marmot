-- What a run was asked, and for each step what it did, what came back and, when it was refused or failed, why and under which rule, so a run reads as a story rather than a list of tool names.
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS prompt TEXT;

ALTER TABLE agent_tool_calls
    ADD COLUMN IF NOT EXISTS detail TEXT,
    ADD COLUMN IF NOT EXISTS result TEXT,
    ADD COLUMN IF NOT EXISTS reason TEXT,
    ADD COLUMN IF NOT EXISTS rule   VARCHAR(255);

---- create above / drop below ----

ALTER TABLE agent_tool_calls
    DROP COLUMN IF EXISTS rule,
    DROP COLUMN IF EXISTS reason,
    DROP COLUMN IF EXISTS result,
    DROP COLUMN IF EXISTS detail;

ALTER TABLE agent_runs DROP COLUMN IF EXISTS prompt;
