import { describe, expect, it } from "vitest";

import {
  byteToAscii,
  byteToHex,
  formatOffset,
  offsetWidthFor,
} from "./byteFormat";

describe("byteToHex", () => {
  it("formats zero", () => {
    expect(byteToHex(0)).toBe("00");
  });

  it("formats single-digit values with a leading zero", () => {
    expect(byteToHex(5)).toBe("05");
    expect(byteToHex(0xf)).toBe("0F");
  });

  it("uses uppercase", () => {
    expect(byteToHex(0xab)).toBe("AB");
  });

  it("handles full byte range", () => {
    expect(byteToHex(0xff)).toBe("FF");
  });
});

describe("formatOffset", () => {
  it("zero-pads to the requested width", () => {
    expect(formatOffset(0)).toBe("00000000");
    expect(formatOffset(0x1234)).toBe("00001234");
  });

  it("supports a custom width", () => {
    expect(formatOffset(0xabcd, 4)).toBe("ABCD");
    expect(formatOffset(0xa, 2)).toBe("0A");
  });

  it("uses uppercase", () => {
    expect(formatOffset(0xdead, 4)).toBe("DEAD");
  });
});

describe("byteToAscii", () => {
  it("returns printable ASCII characters literally", () => {
    expect(byteToAscii(0x41)).toBe("A");
    expect(byteToAscii(0x7a)).toBe("z");
    expect(byteToAscii(0x20)).toBe(" ");
    expect(byteToAscii(0x7e)).toBe("~");
  });

  it("returns a dot for control characters", () => {
    expect(byteToAscii(0x00)).toBe(".");
    expect(byteToAscii(0x09)).toBe(".");
    expect(byteToAscii(0x1f)).toBe(".");
  });

  it("returns a dot for high-bit bytes", () => {
    expect(byteToAscii(0x7f)).toBe(".");
    expect(byteToAscii(0x80)).toBe(".");
    expect(byteToAscii(0xff)).toBe(".");
  });
});

describe("offsetWidthFor", () => {
  it("returns at least 4 for tiny or empty buffers", () => {
    expect(offsetWidthFor(0)).toBe(4);
    expect(offsetWidthFor(1)).toBe(4);
    expect(offsetWidthFor(0xff)).toBe(4);
  });

  it("scales with buffer size", () => {
    expect(offsetWidthFor(0x10000)).toBe(4);
    expect(offsetWidthFor(0x10001)).toBe(6);
    expect(offsetWidthFor(0x100_0000)).toBe(6);
    expect(offsetWidthFor(0x1_0000_0000)).toBe(8); // 4 GiB
  });

  it("always returns an even number", () => {
    for (let n = 0; n < 100; n++) {
      const w = offsetWidthFor(2 ** n);
      expect(w % 2).toBe(0);
    }
  });
});
