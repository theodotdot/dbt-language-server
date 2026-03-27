select
    order_id,
    status
from
{% if target.name == 'prod' %}
    {{ ref('orders') }}
{% else %}
    {{ ref('stg_orders') }}
{% endif %}
