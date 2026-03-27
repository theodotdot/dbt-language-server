with order_totals as (
    select
        customer_id,
        count(order_id) as order_count
    from {{ ref('stg_orders') }}
    group by customer_id
),

customer_info as (
    select
        customer_id,
        first_name
    from {{ ref('stg_customers') }}
)

select
    ci.customer_id,
    ci.first_name,
    ot.order_count
from customer_info ci
join order_totals ot on ci.customer_id = ot.customer_id
