meta:
  id: zb_inst_pos_repeat
  endian: le
seq:
  - id: count
    type: u4
instances:
  entries:
    pos: 8
    type: entry
    repeat: expr
    repeat-expr: count
types:
  entry:
    seq:
      - id: val
        type: u2
