-- Create some sample data tables
CREATE TABLE raw_customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE raw_orders (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES raw_customers(id),
    amount DECIMAL(10,2),
    order_date DATE,
    status VARCHAR(20)
);

-- Insert sample data
INSERT INTO raw_customers (name, email) VALUES
    ('Alice Johnson', 'alice@example.com'),
    ('Bob Smith', 'bob@example.com'),
    ('Carol Davis', 'carol@example.com');

INSERT INTO raw_orders (customer_id, amount, order_date, status) VALUES
    (1, 99.99, '2024-01-15', 'completed'),
    (1, 149.50, '2024-02-01', 'completed'),
    (2, 75.25, '2024-01-20', 'completed'),
    (3, 200.00, '2024-02-05', 'pending');
