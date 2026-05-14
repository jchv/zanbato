import { describe, expect, it, vi } from "vitest";

import { PieceTable, type PieceTableChange } from "./pieceTable";

function bytes(...vs: number[]): Uint8Array {
  return new Uint8Array(vs);
}

function ascii(s: string): Uint8Array {
  return new TextEncoder().encode(s);
}

function decode(b: Uint8Array): string {
  return new TextDecoder().decode(b);
}

describe("PieceTable", () => {
  describe("construction", () => {
    it("empty by default", () => {
      const pt = new PieceTable();
      expect(pt.length).toBe(0);
      expect(pt.version).toBe(0);
      expect(pt.toBytes()).toEqual(bytes());
    });

    it("wraps an initial buffer", () => {
      const pt = new PieceTable(ascii("hello"));
      expect(pt.length).toBe(5);
      expect(decode(pt.toBytes())).toBe("hello");
    });

    it("copies the initial buffer (caller mutation is isolated)", () => {
      const initial = ascii("hello");
      const pt = new PieceTable(initial);
      initial[0] = 0x58; // 'X'
      expect(decode(pt.toBytes())).toBe("hello");
    });
  });

  describe("read", () => {
    it("reads a range", () => {
      const pt = new PieceTable(ascii("hello"));
      expect(decode(pt.read(1, 3))).toBe("ell");
    });

    it("readByte returns the byte at offset", () => {
      const pt = new PieceTable(ascii("hello"));
      expect(pt.readByte(0)).toBe(0x68); // 'h'
      expect(pt.readByte(4)).toBe(0x6f); // 'o'
    });

    it("zero-length read returns empty", () => {
      const pt = new PieceTable(ascii("hello"));
      expect(pt.read(2, 0)).toEqual(bytes());
    });

    it("throws on out-of-range read", () => {
      const pt = new PieceTable(ascii("hello"));
      expect(() => pt.read(0, 6)).toThrow(RangeError);
      expect(() => pt.read(6, 0)).toThrow(RangeError);
      expect(() => pt.readByte(5)).toThrow(RangeError);
      expect(() => pt.readByte(-1)).toThrow(RangeError);
    });

    it("chunks yields zero-copy views that cover the range", () => {
      const pt = new PieceTable(ascii("hello world"));
      // Force a multi-piece state by inserting in the middle.
      pt.insert(5, ascii(","));
      const chunks = Array.from(pt.chunks(0, pt.length));
      const reassembled = new Uint8Array(pt.length);
      let pos = 0;
      for (const c of chunks) {
        reassembled.set(c.buffer.subarray(c.offset, c.offset + c.length), pos);
        pos += c.length;
      }
      expect(decode(reassembled)).toBe("hello, world");
    });
  });

  describe("insert", () => {
    it("at the start", () => {
      const pt = new PieceTable(ascii("world"));
      pt.insert(0, ascii("hello "));
      expect(decode(pt.toBytes())).toBe("hello world");
    });

    it("at the end", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.insert(5, ascii(" world"));
      expect(decode(pt.toBytes())).toBe("hello world");
    });

    it("in the middle (splits a piece)", () => {
      const pt = new PieceTable(ascii("hello world"));
      pt.insert(5, ascii(","));
      expect(decode(pt.toBytes())).toBe("hello, world");
    });

    it("into an empty table", () => {
      const pt = new PieceTable();
      pt.insert(0, ascii("hello"));
      expect(decode(pt.toBytes())).toBe("hello");
    });

    it("zero-length is a no-op", () => {
      const pt = new PieceTable(ascii("hello"));
      const before = pt.version;
      pt.insert(2, bytes());
      expect(pt.version).toBe(before);
      expect(decode(pt.toBytes())).toBe("hello");
    });

    it("throws on negative or past-end offset", () => {
      const pt = new PieceTable(ascii("hello"));
      expect(() => pt.insert(-1, ascii("x"))).toThrow(RangeError);
      expect(() => pt.insert(6, ascii("x"))).toThrow(RangeError);
    });

    it("caller can mutate the source bytes after insert without affecting the table", () => {
      const pt = new PieceTable(ascii("hello"));
      const src = ascii("XYZ");
      pt.insert(5, src);
      src[0] = 0x21; // '!'
      expect(decode(pt.toBytes())).toBe("helloXYZ");
    });
  });

  describe("delete", () => {
    it("from the start", () => {
      const pt = new PieceTable(ascii("hello world"));
      pt.delete(0, 6);
      expect(decode(pt.toBytes())).toBe("world");
    });

    it("from the end", () => {
      const pt = new PieceTable(ascii("hello world"));
      pt.delete(5, 6);
      expect(decode(pt.toBytes())).toBe("hello");
    });

    it("from the middle", () => {
      const pt = new PieceTable(ascii("hello world"));
      pt.delete(5, 1);
      expect(decode(pt.toBytes())).toBe("helloworld");
    });

    it("spanning multiple pieces", () => {
      const pt = new PieceTable(ascii("ABCDE"));
      pt.insert(2, ascii("xyz")); // ABxyzCDE
      pt.insert(4, ascii("__")); // ABxy__zCDE
      pt.delete(3, 4); // remove 'y__z'
      expect(decode(pt.toBytes())).toBe("ABxCDE");
    });

    it("entire buffer", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.delete(0, 5);
      expect(pt.length).toBe(0);
      expect(pt.toBytes()).toEqual(bytes());
    });

    it("zero-length is a no-op", () => {
      const pt = new PieceTable(ascii("hello"));
      const before = pt.version;
      pt.delete(2, 0);
      expect(pt.version).toBe(before);
      expect(decode(pt.toBytes())).toBe("hello");
    });

    it("throws on out-of-range delete", () => {
      const pt = new PieceTable(ascii("hello"));
      expect(() => pt.delete(0, 6)).toThrow(RangeError);
      expect(() => pt.delete(-1, 1)).toThrow(RangeError);
      expect(() => pt.delete(2, -1)).toThrow(RangeError);
    });
  });

  describe("overwrite", () => {
    it("same length replacement", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.overwrite(1, ascii("ELL"));
      expect(decode(pt.toBytes())).toBe("hELLo");
    });

    it("shorter replacement (data deleted)", () => {
      const pt = new PieceTable(ascii("hello"));
      // overwrite 1 byte starting at 1 - net length stays 5
      pt.overwrite(1, ascii("X"));
      expect(decode(pt.toBytes())).toBe("hXllo");
    });

    it("extends past the end", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.overwrite(3, ascii("LLOWORLD"));
      expect(decode(pt.toBytes())).toBe("helLLOWORLD");
    });

    it("at exactly the end (pure append)", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.overwrite(5, ascii(" world"));
      expect(decode(pt.toBytes())).toBe("hello world");
    });

    it("single edit step (one undo)", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.overwrite(1, ascii("ELL"));
      expect(pt.canUndo).toBe(true);
      pt.undo();
      expect(decode(pt.toBytes())).toBe("hello");
      expect(pt.canUndo).toBe(false);
    });
  });

  describe("undo/redo", () => {
    it("undo reverses an insert", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.insert(5, ascii(" world"));
      pt.undo();
      expect(decode(pt.toBytes())).toBe("hello");
    });

    it("undo reverses a delete", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.delete(1, 3);
      expect(decode(pt.toBytes())).toBe("ho");
      pt.undo();
      expect(decode(pt.toBytes())).toBe("hello");
    });

    it("multiple undos walk back through history", () => {
      const pt = new PieceTable(ascii("a"));
      pt.insert(1, ascii("b"));
      pt.insert(2, ascii("c"));
      pt.insert(3, ascii("d"));
      expect(decode(pt.toBytes())).toBe("abcd");
      pt.undo();
      expect(decode(pt.toBytes())).toBe("abc");
      pt.undo();
      expect(decode(pt.toBytes())).toBe("ab");
      pt.undo();
      expect(decode(pt.toBytes())).toBe("a");
      expect(pt.canUndo).toBe(false);
    });

    it("redo replays an undone edit", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.insert(5, ascii("!"));
      pt.undo();
      expect(decode(pt.toBytes())).toBe("hello");
      expect(pt.canRedo).toBe(true);
      pt.redo();
      expect(decode(pt.toBytes())).toBe("hello!");
    });

    it("new edit clears the redo stack", () => {
      const pt = new PieceTable(ascii("hello"));
      pt.insert(5, ascii("!"));
      pt.undo();
      expect(pt.canRedo).toBe(true);
      pt.insert(5, ascii("?"));
      expect(pt.canRedo).toBe(false);
      expect(decode(pt.toBytes())).toBe("hello?");
    });

    it("undo with empty stack is a no-op returning false", () => {
      const pt = new PieceTable(ascii("hello"));
      expect(pt.undo()).toBe(false);
      expect(decode(pt.toBytes())).toBe("hello");
    });

    it("redo with empty stack is a no-op returning false", () => {
      const pt = new PieceTable(ascii("hello"));
      expect(pt.redo()).toBe(false);
    });
  });

  describe("subscriptions", () => {
    it("notifies listeners with edit info", () => {
      const pt = new PieceTable(ascii("hello"));
      const events: PieceTableChange[] = [];
      pt.subscribe((e) => events.push(e));

      pt.insert(5, ascii("!"));
      pt.delete(0, 1);
      pt.overwrite(0, ascii("E"));

      expect(events).toEqual([
        { offset: 5, removedLength: 0, insertedLength: 1, version: 1 },
        { offset: 0, removedLength: 1, insertedLength: 0, version: 2 },
        { offset: 0, removedLength: 1, insertedLength: 1, version: 3 },
      ]);
    });

    it("unsubscribe stops notifications", () => {
      const pt = new PieceTable(ascii("hello"));
      const fn = vi.fn();
      const unsub = pt.subscribe(fn);
      pt.insert(5, ascii("!"));
      expect(fn).toHaveBeenCalledTimes(1);
      unsub();
      pt.insert(6, ascii("?"));
      expect(fn).toHaveBeenCalledTimes(1);
    });

    it("undo/redo fire coarse events covering the whole buffer", () => {
      const pt = new PieceTable(ascii("hello"));
      const events: PieceTableChange[] = [];
      pt.subscribe((e) => events.push(e));

      pt.insert(5, ascii(" world")); // length 5 -> 11
      events.length = 0;

      pt.undo(); // length 11 -> 5
      pt.redo(); // length 5 -> 11

      expect(events).toEqual([
        { offset: 0, removedLength: 11, insertedLength: 5, version: 2 },
        { offset: 0, removedLength: 5, insertedLength: 11, version: 3 },
      ]);
    });

    it("version increments on every edit", () => {
      const pt = new PieceTable(ascii("hello"));
      const v0 = pt.version;
      pt.insert(5, ascii("!"));
      const v1 = pt.version;
      pt.delete(0, 1);
      const v2 = pt.version;
      expect(v1).toBeGreaterThan(v0);
      expect(v2).toBeGreaterThan(v1);
    });
  });

  describe("integration / fuzz-ish", () => {
    it("many interleaved edits stay consistent with a reference array", () => {
      // Random but seeded operations: compare against a plain Array<number>
      // as the source of truth.
      let seed = 0xc0ffee;
      const rand = () => {
        // xorshift32
        seed ^= seed << 13;
        seed ^= seed >>> 17;
        seed ^= seed << 5;
        return (seed >>> 0) / 0x100000000;
      };

      const initial = new Uint8Array(16);
      for (let i = 0; i < initial.length; i++) initial[i] = i;

      const pt = new PieceTable(initial);
      const ref: number[] = Array.from(initial);

      for (let step = 0; step < 200; step++) {
        const op = Math.floor(rand() * 3);
        if (op === 0) {
          // insert
          const off = Math.floor(rand() * (ref.length + 1));
          const n = 1 + Math.floor(rand() * 5);
          const ins = new Uint8Array(n);
          for (let i = 0; i < n; i++) ins[i] = Math.floor(rand() * 256);
          pt.insert(off, ins);
          ref.splice(off, 0, ...ins);
        } else if (op === 1 && ref.length > 0) {
          // delete
          const off = Math.floor(rand() * ref.length);
          const n = 1 + Math.floor(rand() * Math.min(4, ref.length - off));
          pt.delete(off, n);
          ref.splice(off, n);
        } else if (op === 2 && ref.length > 0) {
          // overwrite
          const off = Math.floor(rand() * ref.length);
          const n = 1 + Math.floor(rand() * 4);
          const ow = new Uint8Array(n);
          for (let i = 0; i < n; i++) ow[i] = Math.floor(rand() * 256);
          pt.overwrite(off, ow);
          const toDelete = Math.min(n, ref.length - off);
          ref.splice(off, toDelete, ...ow);
        }

        expect(pt.length).toBe(ref.length);
        expect(Array.from(pt.toBytes())).toEqual(ref);
      }
    });

    it("undo all the way back reaches the initial state byte-for-byte", () => {
      const initial = ascii("the quick brown fox");
      const pt = new PieceTable(initial);
      pt.insert(0, ascii(">>>"));
      pt.delete(8, 5);
      pt.overwrite(2, ascii("__"));
      pt.insert(pt.length, ascii("<<<"));
      while (pt.canUndo) pt.undo();
      expect(pt.toBytes()).toEqual(initial);
    });
  });
});
