-- Customer dimension table
{{ config(materialized='table') }}

with customer_orders as (
    select
        customer_id,
        min(order_date) as first_order_date,
        count(*) as total_orders,
        sum(amount) as total_spent
    from {{ source('public', 'raw_orders') }}
    where status = 'completed'
    group by customer_id
)

select
    c.id as customer_id,
    c.name as customer_name,
    c.email as customer_email,
    co.first_order_date,
    coalesce(co.total_orders, 0) as total_orders,
    coalesce(co.total_spent, 0) as total_spent
from {{ source('public', 'raw_customers') }} c
left join customer_orders co on c.id = co.customer_id
