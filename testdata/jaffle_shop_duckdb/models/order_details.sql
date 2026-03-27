select
    o.order_id,
    c.first_name,
    p.payment_method
from {{ ref('orders') }} o
join {{ ref('customers') }} c on o.customer_id = c.customer_id
join {{ ref('stg_payments') }} p on o.order_id = p.order_id
