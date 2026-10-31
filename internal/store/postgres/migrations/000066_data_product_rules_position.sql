-- People order a data product's rules by hand, so the order is stored as an
-- explicit position rather than inferred from priority, which API clients
-- already set for their own purposes. Existing rules keep the order they are
-- shown in today.
ALTER TABLE data_product_rules ADD COLUMN IF NOT EXISTS position INT NOT NULL DEFAULT 0;

UPDATE data_product_rules r
SET position = ordered.rn
FROM (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY data_product_id ORDER BY priority ASC, created_at ASC) - 1 AS rn
    FROM data_product_rules
) ordered
WHERE r.id = ordered.id;

CREATE INDEX IF NOT EXISTS idx_data_product_rules_position ON data_product_rules (data_product_id, position);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_data_product_rules_position;
ALTER TABLE data_product_rules DROP COLUMN IF EXISTS position;
