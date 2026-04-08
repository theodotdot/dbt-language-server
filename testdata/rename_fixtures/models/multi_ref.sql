select * from {{ ref('orders') }}
join {{ ref('customers') }} on true
union all
select * from {{ ref('orders') }}
