CREATE TABLE IF NOT EXISTS team_webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL CHECK (provider IN ('slack', 'discord', 'generic')),
    webhook_url TEXT NOT NULL,
    notification_types JSONB NOT NULL DEFAULT '[]',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_triggered_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_team_webhooks_team_id ON team_webhooks(team_id);
CREATE INDEX IF NOT EXISTS idx_team_webhooks_team_enabled ON team_webhooks(team_id, enabled);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_team_webhook_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-update updated_at
CREATE TRIGGER team_webhooks_updated_at
    BEFORE UPDATE ON team_webhooks
    FOR EACH ROW
    EXECUTE FUNCTION update_team_webhook_updated_at();
