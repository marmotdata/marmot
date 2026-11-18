-- The source database the Postgres plugin catalogues and DuckDB attaches.
CREATE TABLE customers (id int PRIMARY KEY, name text NOT NULL, region text);
CREATE TABLE orders (id int PRIMARY KEY, customer_id int REFERENCES customers(id), total numeric(10,2), created_at timestamptz DEFAULT now());
INSERT INTO customers VALUES (1, 'Ada', 'eu'), (2, 'Grace', 'us'), (3, 'Linus', 'eu');
INSERT INTO orders (id, customer_id, total) VALUES (1, 1, 10.50), (2, 2, 20.00), (3, 1, 7.25), (4, 3, 99.99);
COMMENT ON TABLE orders IS 'Customer orders';
