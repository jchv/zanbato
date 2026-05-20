meta:
  id: zb_expr_sizeof
  endian: le
seq:
  - id: a
    type: u1
  - id: b
    type: u2
  - id: c
    type: u1
  - id: nested
    type: inner
types:
  inner:
    seq:
      - id: d
        type: u4
  bit_aligned:
    seq:
      - id: hi
        type: b3
      - id: lo
        type: b5
  bit_unaligned:
    seq:
      - id: hi
        type: b3
      - id: lo
        type: b4
instances:
  sizeof_a:
    value: a._sizeof
  sizeof_b:
    value: b._sizeof
  sizeof_nested:
    value: nested._sizeof
  sizeof_u1_t:
    value: sizeof<u1>
  sizeof_u4_t:
    value: sizeof<u4>
  sizeof_inner_t:
    value: sizeof<inner>
  sizeof_b3:
    value: sizeof<b3>
  sizeof_b16:
    value: sizeof<b16>
  sizeof_bit_aligned:
    value: sizeof<bit_aligned>
  sizeof_bit_unaligned:
    value: sizeof<bit_unaligned>
