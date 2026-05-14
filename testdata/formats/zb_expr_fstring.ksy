meta:
  id: zb_expr_fstring
  encoding: ASCII
seq:
  - id: name
    type: strz
  - id: val
    type: u1
  - id: x
    type: u1
  - id: y
    type: u1
instances:
  greeting:
    value: '"hello_" + name'
  sum_label:
    value: '"sum=" + (x + y).to_s'
