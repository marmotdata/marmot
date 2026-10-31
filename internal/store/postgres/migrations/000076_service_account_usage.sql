-- Calls per service account per day, bumped in the same statement that already records a key's last use, so the detail page can show recent activity without a request log. A day bucket keeps the table to one row per account per active day.
CREATE TABLE IF NOT EXISTS service_account_usage_daily (
    service_account_id UUID   NOT NULL REFERENCES service_accounts(id) ON DELETE CASCADE,
    day                DATE   NOT NULL,
    calls              BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (service_account_id, day)
);

-- Agents report runs with a service account key, so recording which account sent a run is what links an account to the agents that use it. Nullable because runs sent with a personal key or before this column have no account.
ALTER TABLE agent_runs
    ADD COLUMN IF NOT EXISTS service_account_id UUID REFERENCES service_accounts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_agent_runs_service_account
    ON agent_runs (service_account_id) WHERE service_account_id IS NOT NULL;

---- create above / drop below ----

DROP INDEX IF EXISTS idx_agent_runs_service_account;
ALTER TABLE agent_runs DROP COLUMN IF EXISTS service_account_id;
DROP TABLE IF EXISTS service_account_usage_daily;
