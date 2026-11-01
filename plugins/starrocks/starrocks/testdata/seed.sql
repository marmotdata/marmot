CREATE DATABASE IF NOT EXISTS shop;
USE shop;

CREATE TABLE events (
  event_date DATE NOT NULL COMMENT "Day the event happened",
  event_id BIGINT NOT NULL COMMENT "Event identifier",
  user_id BIGINT NULL,
  event_type VARCHAR(64) NULL,
  payload JSON NULL COMMENT "Raw event payload",
  tag_ids ARRAY<INT> NULL COMMENT "Tag identifiers"
)
DUPLICATE KEY(event_date, event_id)
COMMENT "Raw clickstream events"
PARTITION BY RANGE(event_date) (
  PARTITION p202601 VALUES [("2026-01-01"), ("2026-02-01")),
  PARTITION p202602 VALUES [("2026-02-01"), ("2026-03-01"))
)
DISTRIBUTED BY HASH(event_id) BUCKETS 4
PROPERTIES ("replication_num" = "1");

CREATE TABLE customers (
  customer_id BIGINT NOT NULL COMMENT "Customer identifier",
  email VARCHAR(255) NOT NULL COMMENT "Customer email address",
  country VARCHAR(64) NULL COMMENT "ISO country code",
  created_at DATETIME NULL
)
PRIMARY KEY(customer_id)
COMMENT "Customer master record"
DISTRIBUTED BY HASH(customer_id) BUCKETS 3
PROPERTIES ("replication_num" = "1");

CREATE TABLE daily_stats (
  stat_date DATE NOT NULL COMMENT "Aggregation day",
  country VARCHAR(64) NOT NULL,
  order_total DECIMAL(10,2) SUM DEFAULT "0" COMMENT "Summed order value",
  max_order DECIMAL(10,2) MAX DEFAULT "0",
  last_channel VARCHAR(64) REPLACE DEFAULT "web"
)
AGGREGATE KEY(stat_date, country)
COMMENT "Per country daily aggregates"
DISTRIBUTED BY HASH(country) BUCKETS 2
PROPERTIES ("replication_num" = "1");

CREATE TABLE orders (
  order_id BIGINT NOT NULL COMMENT "Order identifier",
  order_date DATE NOT NULL,
  customer_id BIGINT NULL,
  amount DECIMAL(10,2) NULL
)
UNIQUE KEY(order_id, order_date)
COMMENT "Orders, latest row wins"
DISTRIBUTED BY HASH(order_id) BUCKETS 3
ORDER BY (order_date, order_id)
PROPERTIES ("replication_num" = "1");

INSERT INTO events VALUES
  ("2026-01-05", 1, 100, "view", parse_json('{"path":"/home"}'), [1,2]),
  ("2026-01-06", 2, 101, "click", parse_json('{"path":"/cart"}'), [3]),
  ("2026-02-03", 3, 100, "purchase", parse_json('{"path":"/pay"}'), [2,4]);

INSERT INTO customers VALUES
  (100, "a@example.com", "NL", "2026-01-01 10:00:00"),
  (101, "b@example.com", "DE", "2026-01-02 11:00:00");

INSERT INTO daily_stats VALUES
  ("2026-01-05", "NL", 120.50, 80.00, "web"),
  ("2026-01-06", "DE", 40.00, 40.00, "app");

INSERT INTO orders VALUES
  (5001, "2026-01-05", 100, 80.00),
  (5002, "2026-01-06", 101, 40.00);

CREATE VIEW daily_sales COMMENT "Daily sales by country" AS
SELECT o.order_date, c.country, SUM(o.amount) AS total
FROM orders o JOIN customers c ON o.customer_id = c.customer_id
GROUP BY o.order_date, c.country;

CREATE MATERIALIZED VIEW mv_sales
COMMENT "Materialized daily sales"
DISTRIBUTED BY HASH(order_date) BUCKETS 2
REFRESH ASYNC
PROPERTIES ("replication_num" = "1")
AS SELECT order_date, SUM(amount) AS total FROM orders GROUP BY order_date;
