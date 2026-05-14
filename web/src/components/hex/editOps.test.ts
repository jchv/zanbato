import { describe, expect, it } from "vitest";

import {
  type EditContext,
  backspace,
  deleteForward,
  isPrintableAscii,
  parseHexDigit,
  typeAscii,
  typeHexDigit,
} from "./editOps";

function ctx(overrides: Partial<EditContext> = {}): EditContext {
  return {
    caret: 5,
    length: 10,
    byteAtCaret: 0xab,
    nibble: "high",
    insertMode: false,
    selectionRange: null,
    ...overrides,
  };
}

describe("parseHexDigit", () => {
  it("parses digits", () => {
    expect(parseHexDigit("0")).toBe(0);
    expect(parseHexDigit("9")).toBe(9);
  });

  it("parses lower and upper case letters", () => {
    expect(parseHexDigit("a")).toBe(10);
    expect(parseHexDigit("f")).toBe(15);
    expect(parseHexDigit("A")).toBe(10);
    expect(parseHexDigit("F")).toBe(15);
  });

  it("rejects non-hex", () => {
    expect(parseHexDigit("g")).toBeNull();
    expect(parseHexDigit("G")).toBeNull();
    expect(parseHexDigit(" ")).toBeNull();
    expect(parseHexDigit("")).toBeNull();
    expect(parseHexDigit("12")).toBeNull();
    expect(parseHexDigit("Enter")).toBeNull();
  });
});

describe("isPrintableAscii", () => {
  it("accepts printable range", () => {
    expect(isPrintableAscii(" ")).toBe(true);
    expect(isPrintableAscii("A")).toBe(true);
    expect(isPrintableAscii("~")).toBe(true);
  });

  it("rejects control chars and named keys", () => {
    expect(isPrintableAscii("\n")).toBe(false);
    expect(isPrintableAscii("\t")).toBe(false);
    expect(isPrintableAscii("Enter")).toBe(false);
    expect(isPrintableAscii("")).toBe(false);
  });
});

describe("typeHexDigit - overwrite, high nibble", () => {
  it("replaces the high nibble and advances to low", () => {
    const r = typeHexDigit(0x5, ctx({ byteAtCaret: 0xab }));
    expect(r.ops).toEqual([
      { kind: "overwrite", offset: 5, data: Uint8Array.of(0x5b) },
    ]);
    expect(r.newCaret).toBe(5);
    expect(r.newNibble).toBe("low");
  });
});

describe("typeHexDigit - overwrite, low nibble", () => {
  it("replaces the low nibble and advances caret", () => {
    const r = typeHexDigit(0xc, ctx({ byteAtCaret: 0x5b, nibble: "low" }));
    expect(r.ops).toEqual([
      { kind: "overwrite", offset: 5, data: Uint8Array.of(0x5c) },
    ]);
    expect(r.newCaret).toBe(6);
    expect(r.newNibble).toBe("high");
  });
});

describe("typeHexDigit - insert mode", () => {
  it("inserts a new byte on the first nibble entry", () => {
    const r = typeHexDigit(
      0x5,
      ctx({ insertMode: true, nibble: "high", byteAtCaret: 0xab }),
    );
    expect(r.ops).toEqual([
      { kind: "insert", offset: 5, data: Uint8Array.of(0x50) },
    ]);
    expect(r.newCaret).toBe(5);
    expect(r.newNibble).toBe("low");
  });

  it("completes the low nibble of the just-inserted byte and advances", () => {
    // After the above, the caret sits on the new byte (0x50). The next
    // keystroke replaces its low nibble.
    const r = typeHexDigit(
      0xc,
      ctx({ insertMode: true, nibble: "low", byteAtCaret: 0x50 }),
    );
    expect(r.ops).toEqual([
      { kind: "overwrite", offset: 5, data: Uint8Array.of(0x5c) },
    ]);
    expect(r.newCaret).toBe(6);
    expect(r.newNibble).toBe("high");
  });
});

describe("typeHexDigit - past end of buffer", () => {
  it("inserts when caret is at length (no byte to overwrite)", () => {
    const r = typeHexDigit(
      0x5,
      ctx({ caret: 10, length: 10, byteAtCaret: undefined }),
    );
    expect(r.ops).toEqual([
      { kind: "insert", offset: 10, data: Uint8Array.of(0x50) },
    ]);
    expect(r.newCaret).toBe(10);
    expect(r.newNibble).toBe("low");
  });
});

describe("typeHexDigit - selection replace", () => {
  it("deletes the selected range, then inserts the new high nibble", () => {
    const r = typeHexDigit(
      0x7,
      ctx({
        caret: 6, // doesn't matter; range takes precedence
        selectionRange: [3, 5],
        byteAtCaret: 0x00,
      }),
    );
    expect(r.ops).toEqual([
      { kind: "delete", offset: 3, count: 3 },
      { kind: "insert", offset: 3, data: Uint8Array.of(0x70) },
    ]);
    expect(r.newCaret).toBe(3);
    expect(r.newNibble).toBe("low");
  });

  it("a selection that covers the tail still appends via insert", () => {
    const r = typeHexDigit(
      0x7,
      ctx({
        length: 10,
        selectionRange: [8, 9],
        caret: 10,
      }),
    );
    // delete [8,9] -> length becomes 8, caret at 8 (== new length), insert.
    expect(r.ops).toEqual([
      { kind: "delete", offset: 8, count: 2 },
      { kind: "insert", offset: 8, data: Uint8Array.of(0x70) },
    ]);
    expect(r.newCaret).toBe(8);
    expect(r.newNibble).toBe("low");
  });
});

describe("typeAscii", () => {
  it("overwrites the byte at caret", () => {
    const r = typeAscii(0x41, ctx({ caret: 3 }));
    expect(r.ops).toEqual([
      { kind: "overwrite", offset: 3, data: Uint8Array.of(0x41) },
    ]);
    expect(r.newCaret).toBe(4);
  });

  it("inserts in insert mode", () => {
    const r = typeAscii(0x41, ctx({ caret: 3, insertMode: true }));
    expect(r.ops).toEqual([
      { kind: "insert", offset: 3, data: Uint8Array.of(0x41) },
    ]);
    expect(r.newCaret).toBe(4);
  });

  it("inserts at the past-end position regardless of mode", () => {
    const r = typeAscii(
      0x41,
      ctx({ caret: 10, length: 10, byteAtCaret: undefined }),
    );
    expect(r.ops).toEqual([
      { kind: "insert", offset: 10, data: Uint8Array.of(0x41) },
    ]);
    expect(r.newCaret).toBe(11);
  });

  it("replaces selection with a single byte", () => {
    const r = typeAscii(0x41, ctx({ selectionRange: [2, 4], caret: 5 }));
    expect(r.ops).toEqual([
      { kind: "delete", offset: 2, count: 3 },
      { kind: "insert", offset: 2, data: Uint8Array.of(0x41) },
    ]);
    expect(r.newCaret).toBe(3);
  });
});

describe("backspace", () => {
  it("deletes the byte before the caret", () => {
    const r = backspace(ctx({ caret: 5 }));
    expect(r.ops).toEqual([{ kind: "delete", offset: 4, count: 1 }]);
    expect(r.newCaret).toBe(4);
  });

  it("at caret 0 is a no-op (no buffer change)", () => {
    const r = backspace(ctx({ caret: 0 }));
    expect(r.ops).toEqual([]);
    expect(r.newCaret).toBe(0);
  });

  it("deletes the selection range and parks the caret at its start", () => {
    const r = backspace(ctx({ selectionRange: [2, 5], caret: 6 }));
    expect(r.ops).toEqual([{ kind: "delete", offset: 2, count: 4 }]);
    expect(r.newCaret).toBe(2);
  });
});

describe("deleteForward", () => {
  it("deletes the byte at the caret, caret stays in place", () => {
    const r = deleteForward(ctx({ caret: 5, length: 10 }));
    expect(r.ops).toEqual([{ kind: "delete", offset: 5, count: 1 }]);
    expect(r.newCaret).toBe(5);
  });

  it("at past-end is a no-op", () => {
    const r = deleteForward(
      ctx({ caret: 10, length: 10, byteAtCaret: undefined }),
    );
    expect(r.ops).toEqual([]);
    expect(r.newCaret).toBe(10);
  });

  it("deletes the selection range", () => {
    const r = deleteForward(ctx({ selectionRange: [2, 5], caret: 5 }));
    expect(r.ops).toEqual([{ kind: "delete", offset: 2, count: 4 }]);
    expect(r.newCaret).toBe(2);
  });
});
