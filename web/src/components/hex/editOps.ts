export type Nibble = "high" | "low";

export type BufferOp =
  | { kind: "insert"; offset: number; data: Uint8Array }
  | { kind: "overwrite"; offset: number; data: Uint8Array }
  | { kind: "delete"; offset: number; count: number };

export interface EditContext {
  caret: number;
  length: number;
  byteAtCaret: number | undefined; // undefined if caret === length
  nibble: Nibble;
  insertMode: boolean;
  selectionRange: [number, number] | null; // null if no selection
}

export interface EditResult {
  ops: BufferOp[];
  newCaret: number;
  newNibble: Nibble;
}

/**
 * Hex digits 0-9, a-f, A-F -> 0..15. Returns null for any other character.
 */
export function parseHexDigit(key: string): number | null {
  if (key.length !== 1) return null;
  const c = key.charCodeAt(0);
  if (c >= 0x30 && c <= 0x39) return c - 0x30; // 0-9
  if (c >= 0x41 && c <= 0x46) return c - 0x41 + 10; // A-F
  if (c >= 0x61 && c <= 0x66) return c - 0x61 + 10; // a-f
  return null;
}

/**
 * Whether a key is a printable ASCII character we should accept in the
 * ASCII column. Single-character keys in 0x20..0x7E.
 */
export function isPrintableAscii(key: string): boolean {
  if (key.length !== 1) return false;
  const c = key.charCodeAt(0);
  return c >= 0x20 && c <= 0x7e;
}

/**
 * Apply a hex digit at the caret.
 */
export function typeHexDigit(digit: number, ctx: EditContext): EditResult {
  const ops: BufferOp[] = [];
  let { caret, length, nibble, byteAtCaret } = ctx;
  const { selectionRange: range, insertMode } = ctx;

  if (range !== null) {
    const [lo, hi] = range;
    const count = hi - lo + 1;
    ops.push({ kind: "delete", offset: lo, count });
    caret = lo;
    length -= count;
    nibble = "high";
    byteAtCaret = undefined; // the byte we'd have overwritten is gone
  }

  // Even in overwrite mode, we should insert if we're replacing a selection or
  // at the end of the file.
  const effectiveInsert = range !== null || insertMode || caret === length;

  if (nibble === "high") {
    if (effectiveInsert) {
      const newByte = (digit & 0xf) << 4;
      ops.push({
        kind: "insert",
        offset: caret,
        data: Uint8Array.of(newByte),
      });
      // Caret stays on the just-inserted byte; next nibble fills its low.
      return { ops, newCaret: caret, newNibble: "low" };
    }
    // Overwrite mode, existing byte: replace high nibble.
    const base = byteAtCaret ?? 0;
    const newByte = ((digit & 0xf) << 4) | (base & 0x0f);
    ops.push({
      kind: "overwrite",
      offset: caret,
      data: Uint8Array.of(newByte),
    });
    return { ops, newCaret: caret, newNibble: "low" };
  }

  // After the high nibble, we always "overwrite" to fill in the low nibble.
  const current = byteAtCaret ?? 0;
  const newByte = (current & 0xf0) | (digit & 0xf);
  ops.push({
    kind: "overwrite",
    offset: caret,
    data: Uint8Array.of(newByte),
  });
  return { ops, newCaret: caret + 1, newNibble: "high" };
}

/**
 * Apply a printable ASCII character at the caret.
 */
export function typeAscii(charCode: number, ctx: EditContext): EditResult {
  const ops: BufferOp[] = [];
  let { caret, length } = ctx;
  const { selectionRange: range, insertMode } = ctx;

  if (range !== null) {
    const [lo, hi] = range;
    const count = hi - lo + 1;
    ops.push({ kind: "delete", offset: lo, count });
    caret = lo;
    length -= count;
  }

  const data = Uint8Array.of(charCode);
  const insert = range !== null || insertMode || caret === length;
  ops.push({ kind: insert ? "insert" : "overwrite", offset: caret, data });
  return { ops, newCaret: caret + 1, newNibble: "high" };
}

export function backspace(ctx: EditContext): EditResult {
  const { caret, selectionRange: range } = ctx;

  if (range !== null) {
    const [lo, hi] = range;
    return {
      ops: [{ kind: "delete", offset: lo, count: hi - lo + 1 }],
      newCaret: lo,
      newNibble: "high",
    };
  }

  if (caret === 0) {
    // Nothing to delete; just reset the nibble pairing.
    return { ops: [], newCaret: 0, newNibble: "high" };
  }

  return {
    ops: [{ kind: "delete", offset: caret - 1, count: 1 }],
    newCaret: caret - 1,
    newNibble: "high",
  };
}

export function deleteForward(ctx: EditContext): EditResult {
  const { caret, length, selectionRange: range } = ctx;

  if (range !== null) {
    const [lo, hi] = range;
    return {
      ops: [{ kind: "delete", offset: lo, count: hi - lo + 1 }],
      newCaret: lo,
      newNibble: "high",
    };
  }

  if (caret >= length) {
    return { ops: [], newCaret: caret, newNibble: "high" };
  }

  return {
    ops: [{ kind: "delete", offset: caret, count: 1 }],
    newCaret: caret,
    newNibble: "high",
  };
}
