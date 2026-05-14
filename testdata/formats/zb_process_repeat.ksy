meta:
  id: zb_process_repeat
seq:
  - id: count
    type: u1
  - id: entries
    size: 2
    repeat: expr
    repeat-expr: count
    process: xor(0xAA)
