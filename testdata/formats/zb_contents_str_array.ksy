meta:
  id: zb_contents_str_array
seq:
  - id: header
    contents:
      - '0xff'    # single byte 0xff
      - '255'     # single byte 0xff (decimal 255)
      - 'abc'     # three bytes: 0x61 0x62 0x63
      - 0xfe      # bare int -> single byte
