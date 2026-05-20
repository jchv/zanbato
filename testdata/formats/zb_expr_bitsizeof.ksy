meta:
  id: zb_expr_bitsizeof
  endian: le
seq:
  - id: pad
    type: u1
types:
  inner:
    seq:
      - id: x
        type: u2
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
  bitsizeof_u1:
    value: bitsizeof<u1>
  bitsizeof_u4:
    value: bitsizeof<u4>
  bitsizeof_inner:
    value: bitsizeof<inner>
  bitsizeof_b1:
    value: bitsizeof<b1>
  bitsizeof_b3:
    value: bitsizeof<b3>
  bitsizeof_b16:
    value: bitsizeof<b16>
  bitsizeof_bit_aligned:
    value: bitsizeof<bit_aligned>
  bitsizeof_bit_unaligned:
    value: bitsizeof<bit_unaligned>
