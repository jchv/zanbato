meta:
  id: zb_expr_arr_methods
  endian: le
seq:
  - id: items
    type: u2
    repeat: expr
    repeat-expr: 5
instances:
  arr_size:
    value: items.size
  arr_first:
    value: items.first
  arr_last:
    value: items.last
  arr_min:
    value: items.min
  arr_max:
    value: items.max
