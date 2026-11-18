{% macro create_raw_orders() %}
create schema if not exists raw;
create or replace table raw.orders as
select * from (values
    (1001, 1, date '2026-01-03', 'shipped', 42.50),
    (1002, 2, date '2026-01-04', 'shipped', 18.00),
    (1003, 1, date '2026-01-07', 'returned', 7.25),
    (1004, 3, date '2026-01-09', 'placed', 99.99),
    (1005, 4, date '2026-01-11', 'shipped', 12.00)
) as t(id, customer_id, ordered_at, status, amount);
{% endmacro %}
