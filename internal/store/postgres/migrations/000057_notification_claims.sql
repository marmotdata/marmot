-- Notices that must go out once per occurrence, such as a key that is about
-- to expire, claim a key here before they are sent. The claim is kept apart
-- from the notifications, so a recipient dismissing theirs, or pruning
-- removing it, never causes the notice to be sent again.
CREATE TABLE IF NOT EXISTS notification_claims (
    key        TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_claims_created_at ON notification_claims (created_at);
