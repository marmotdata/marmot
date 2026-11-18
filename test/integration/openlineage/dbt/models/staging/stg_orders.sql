select
    id as order_id,
    customer_id,
    ordered_at,
    status,
    amount
from {{ source('raw', 'orders') }}
