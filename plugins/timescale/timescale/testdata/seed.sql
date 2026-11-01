-- Fixture for the end to end tests. Load it into a fresh container with:
--
--   docker exec -i marmot-test-timescale psql -U postgres -d shop < seed.sql
--
-- generate_series over three days at a 30 minute step produces 145 rows and,
-- at a one day chunk interval across four space partitions, several chunks.

CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE customers (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);
COMMENT ON TABLE customers IS 'Shop customers';
COMMENT ON COLUMN customers.email IS 'Contact email address';

CREATE TABLE conditions (
  time TIMESTAMPTZ NOT NULL,
  device_id INT,
  temperature DOUBLE PRECISION,
  humidity DOUBLE PRECISION
);
COMMENT ON TABLE conditions IS 'Sensor readings';

SELECT create_hypertable('conditions', 'time', chunk_time_interval => INTERVAL '1 day');
SELECT add_dimension('conditions', 'device_id', number_partitions => 4);

INSERT INTO customers (name, email)
VALUES ('alice', 'alice@example.com'), ('bob', NULL), ('carol', 'carol@example.com');

INSERT INTO conditions (time, device_id, temperature, humidity)
SELECT ts, (random() * 3)::int + 1, 20 + random() * 10, 40 + random() * 20
FROM generate_series(now() - INTERVAL '3 days', now(), INTERVAL '30 minutes') AS ts;

ALTER TABLE conditions SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'device_id',
  timescaledb.compress_orderby = 'time DESC'
);
SELECT add_compression_policy('conditions', INTERVAL '7 days');
SELECT add_retention_policy('conditions', INTERVAL '90 days');

CREATE MATERIALIZED VIEW conditions_daily
WITH (timescaledb.continuous) AS
SELECT time_bucket('1 day', time) AS bucket, device_id, avg(temperature) AS avg_temp
FROM conditions
GROUP BY 1, 2;

SELECT add_continuous_aggregate_policy('conditions_daily',
  start_offset => INTERVAL '30 days',
  end_offset => INTERVAL '1 hour',
  schedule_interval => INTERVAL '1 hour');

ALTER TABLE conditions ADD COLUMN customer_id INT;
ALTER TABLE conditions ADD CONSTRAINT conditions_customer_fk
  FOREIGN KEY (customer_id) REFERENCES customers(id);

CREATE VIEW customer_readings AS
SELECT c.name, co.time, co.temperature
FROM customers c
JOIN conditions co ON co.customer_id = c.id;

ANALYZE;
