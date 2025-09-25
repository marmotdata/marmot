-- Customer analytics summary
{{ config(materialized='table') }}

with delayed_data as (
    select
        c.customer_id,
        c.customer_name,
        c.total_orders,
        c.total_spent,
        gs.n,
        -- expensive computation to make pipeline longer
        sin(gs.n::float) * cos(gs.n::float) as calc
    from {{ ref('customers') }} c
    cross join generate_series(1, 10000000) as gs(n)
    where c.total_orders > 0
),
aggregated as (
    select
        customer_id,
        customer_name,
        total_orders,
        total_spent,
        sum(calc) as total_calc
    from delayed_data
    group by customer_id, customer_name, total_orders, total_spent
)
select
    customer_id,
    customer_name,
    total_orders,
    total_spent,
    case 
        when total_orders > 0 then total_spent / total_orders
        else 0
    end as avg_order_value
from aggregated
