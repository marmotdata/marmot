select
    id as customer_id,
    first_name || ' ' || last_name as customer_name,
    country
from {{ ref('raw_customers') }}
