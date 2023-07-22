# Zanbato

Zanbato is a compiler implementation for Kaitai Struct .ksy files written in Go and mainly targeting Go. It is still under development, but it does support enough Kaitai Struct functionality to be potentially useful.

It is not intended to replace the upstream Kaitai Struct compiler, it is just meant to be an alternate implementation that may be of interest in pure Go projects or as a potentially slightly easier way to mess around with .ksy definitions (although note that I would consider this library to be fairly poorly designed right now and in need of some clean up. Please help if you have ideas!)

## Status

Some complicated Kaitai Struct definitions will work, however, proceed with caution, as incorrect or malformed code is likely to occur still. I have not tested this against the upstream Kaitai test suite at all.

Here are some of the features that do work:

- Structures
  + ✅ Basic data types (integers, strings, bytes, etc.)
  + ✅ Type switches
  + ✅ Endianness
    - ✅ Inheriting endianness
    - ⚠️ **NOTE: Endian switching is not supported.**
  + ✅ Referring to other types
  + ✅ Repeating:
    - ✅ Repeat count of iterations
    - ✅ Repeat until end of stream
    - ⚠️ **NOTE: Repeat until expression is not supported**
- ✅ Enumerations
- ✅ Parameters
- ✅ Expressions
  + ⚠️ **Only basic literals and limited field access.** The groundwork for parsing expressions is present but unfinished.
  + ⚠️ **In addition, special variables are not implemented.**
