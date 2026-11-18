-- Only built by the failure scenario in run.sh: the column does not exist, so the model fails at run time and dbt-ol emits a FAIL event.
{{ config(enabled=var('include_broken')) }}
select order_id, no_such_column
from {{ ref('stg_orders') }}
