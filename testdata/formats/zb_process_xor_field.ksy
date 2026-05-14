meta:
  id: zb_process_xor_field
seq:
  - id: key
    type: u1
  - id: data
    size: 8
    process: xor(key)
