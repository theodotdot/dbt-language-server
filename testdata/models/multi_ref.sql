select
    a.id,
    b.name
from {{ ref('orders') }} a
join {{ ref('customers') }} b on a.customer_id = b.id
join {{ ref('orders') }} c on a.id = c.id
