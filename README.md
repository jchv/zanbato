[Web Editor](https://jchv.github.io/zanbato/) &bull; [![Go Report Card](https://goreportcard.com/badge/github.com/jchv/zanbato)](https://goreportcard.com/report/github.com/jchv/zanbato) &bull; [![CI](https://github.com/jchv/zanbato/actions/workflows/ci.yml/badge.svg)](https://github.com/jchv/zanbato/actions/workflows/ci.yml)

# Zanbato

Zanbato is a compiler and runtime evaluator implementation for Kaitai Struct .ksy files written in Go that currently targets Go and C. It has fairly complete support for the features of Kaitai Struct.

It is not intended to replace the upstream Kaitai Struct compiler. It is an alternate implementation that may be useful in pure Go projects, for experimenting with `.ksy` definitions, and for tooling that wants Kaitai Struct support without depending on the JVM-based compiler.

## Status

Zanbato currently passes the full upstream Kaitai Struct test suite, plus additional tests created to try to shake out bugs in Zanbato specifically that are also tested against upstream Kaitai Struct. It is still under active development, so there may very well be bugs, especially in lesser-used functionality.

All of the Kaitai Struct features should work to some degree, including:

- ✅ Structures
  + ✅ Basic data types (integers, strings, bits, bytes, etc.)
  + ✅ Type switches
  + ✅ Endianness
    - ✅ Inheriting endianness
    - ✅ Endian switching
    - ✅ Bit-endianness
  + ✅ Referring to other types
  + ✅ Repeating:
    - ✅ Repeat count of iterations
    - ✅ Repeat until end of stream
    - ✅ Repeat until expression is true
- ✅ Enumerations
- ✅ Parameters
- ✅ Expressions
  + ✅ Unary, binary, and ternary operators
  + ✅ Common string, byte-array, array, enum, and stream helper methods
- ✅ Instances
  + ✅ Value instances
  + ✅ Positioned instances
  + ✅ Conditional instances
- ✅ Serialization
  + Please note that serialization is a WIP and much less tested.
  + Both Go and C emitters support serialization.
  + The runtime evaluator only supports modifying scalars.
