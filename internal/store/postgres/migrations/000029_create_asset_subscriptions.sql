CREATE TABLE IF NOT EXISTS asset_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id VARCHAR(255) NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_types JSONB NOT NULL DEFAULT '["asset_change", "schema_change"]',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (asset_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_asset_subscriptions_user ON asset_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_asset_subscriptions_asset ON asset_subscriptions(asset_id);
CREATE INDEX IF NOT EXISTS idx_asset_subscriptions_types ON asset_subscriptions USING gin (notification_types);

---- create above / drop below ----

DROP TABLE IF EXISTS asset_subscriptions;
