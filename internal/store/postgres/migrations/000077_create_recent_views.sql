-- What each person opened last, so Home can offer it back to them. One row per
-- person and item, with the time refreshed on every view, so the table stays
-- bounded by the number of distinct items a person keeps (the service trims it
-- to the latest few dozen). Items are not foreign keys because they span
-- several tables; reads join to each and drop items that have since gone.
CREATE TABLE IF NOT EXISTS recent_views (
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind      VARCHAR(20) NOT NULL CHECK (kind IN ('asset', 'glossary_term', 'data_product', 'saved_query')),
    item_id   TEXT NOT NULL,
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, kind, item_id)
);

CREATE INDEX IF NOT EXISTS idx_recent_views_user_viewed ON recent_views (user_id, viewed_at DESC);

---- create above / drop below ----

DROP TABLE IF EXISTS recent_views;
