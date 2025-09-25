-- Clean orders fact table
{{ config(materialized='table') }}

select
    id as order_id,
    customer_id,
    amount as order_amount,
    order_date,
    status as order_status
from {{ source('public', 'raw_orders') }}
where order_date is not null
