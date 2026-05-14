meta:
  id: zb_params_basic
seq:
  - id: data_len
    type: u1
  - id: body
    type: payload(data_len)
types:
  payload:
    params:
      - id: len
        type: u4
    seq:
      - id: content
        size: len
