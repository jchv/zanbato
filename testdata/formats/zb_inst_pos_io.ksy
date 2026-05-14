meta:
  id: zb_inst_pos_io
  endian: le
seq:
  - id: header_len
    type: u4
  - id: data_offset
    type: u4
instances:
  payload:
    pos: data_offset
    size: 5
