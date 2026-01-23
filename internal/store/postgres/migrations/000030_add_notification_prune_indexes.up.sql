-- Partial index for pruning read notifications by age
CREATE INDEX IF NOT EXISTS idx_notifications_read_created_at
ON notifications(created_at) WHERE read = true;

-- Index for per-user notification cap enforcement
CREATE INDEX IF NOT EXISTS idx_notifications_user_id_created_at_desc
ON notifications(user_id, created_at DESC);
