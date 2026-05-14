meta:
  id: zb_inst_array
  endian: le
seq:
  - id: count
    type: u1
  - id: values
    type: u2
    repeat: expr
    repeat-expr: count
instances:
  total_elements:
    value: values.size
  first_val:
    value: values.first
  last_val:
    value: values.last
