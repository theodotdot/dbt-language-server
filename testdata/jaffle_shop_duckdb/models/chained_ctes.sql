with base_orders as (
    select * from {{ ref('stg_orders') }}
),

filtered_orders as (
    select
        order_id,
        status
    from base_orders
    where status != 'returned'
),

final as (
    select * from filtered_orders
)

select * from final
