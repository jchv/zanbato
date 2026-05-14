meta:
  id: zb_switch_on_calc
  endian: le
seq:
  - id: a
    type: u1
  - id: b
    type: u1
  - id: body
    type:
      switch-on: a + b
      cases:
        3: payload_s
        5: payload_l
types:
  payload_s:
    seq:
      - id: val
        type: u2
  payload_l:
    seq:
      - id: val
        type: u4
