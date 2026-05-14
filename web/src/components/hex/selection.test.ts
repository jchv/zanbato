import { describe, expect, it } from "vitest";

import {
  type SelectionState,
  isInSelection,
  moveCaret,
  selectionRange,
} from "./selection";

function sel(
  anchor: number,
  caret: number,
  column: "hex" | "ascii" = "hex",
): SelectionState {
  return { anchor, caret, column };
}

const OPTS = {
  length: 100,
  columns: 16,
  visibleRows: 4,
  ctrl: false,
  shift: false,
};

describe("selectionRange", () => {
  it("returns null when collapsed", () => {
    expect(selectionRange(sel(5, 5))).toBeNull();
  });

  it("orders anchor and caret, excluding the past-end caret position", () => {
    // anchor=3, caret=7 -> bytes 3..6 selected (4 bytes), caret sits past byte 6
    expect(selectionRange(sel(3, 7))).toEqual([3, 6]);
    expect(selectionRange(sel(7, 3))).toEqual([3, 6]);
  });
});

describe("isInSelection", () => {
  it("includes start, excludes caret-end position", () => {
    const s = sel(3, 7);
    expect(isInSelection(3, s)).toBe(true);
    expect(isInSelection(5, s)).toBe(true);
    expect(isInSelection(6, s)).toBe(true);
    expect(isInSelection(7, s)).toBe(false); // caret sits past byte 6
    expect(isInSelection(2, s)).toBe(false);
  });

  it("is empty for a collapsed selection", () => {
    expect(isInSelection(5, sel(5, 5))).toBe(false);
  });

  it("works regardless of anchor/caret order", () => {
    expect(isInSelection(5, sel(7, 3))).toBe(true);
  });
});

describe("moveCaret - collapsed (no shift)", () => {
  it("ArrowLeft moves left, clamps at 0", () => {
    expect(moveCaret(sel(5, 5), "ArrowLeft", OPTS)).toEqual(sel(4, 4));
    expect(moveCaret(sel(0, 0), "ArrowLeft", OPTS)).toEqual(sel(0, 0));
  });

  it("ArrowRight moves right, can reach the past-end position", () => {
    expect(moveCaret(sel(5, 5), "ArrowRight", OPTS)).toEqual(sel(6, 6));
    expect(moveCaret(sel(99, 99), "ArrowRight", OPTS)).toEqual(sel(100, 100));
    expect(moveCaret(sel(100, 100), "ArrowRight", OPTS)).toEqual(sel(100, 100));
  });

  it("ArrowDown jumps a full row", () => {
    expect(moveCaret(sel(5, 5), "ArrowDown", OPTS)).toEqual(sel(21, 21));
  });

  it("ArrowUp jumps back a row but stays in column", () => {
    expect(moveCaret(sel(21, 21), "ArrowUp", OPTS)).toEqual(sel(5, 5));
  });

  it("ArrowUp from the first row clamps to the same column on row 0", () => {
    expect(moveCaret(sel(5, 5), "ArrowUp", OPTS)).toEqual(sel(5, 5));
  });

  it("Home goes to the start of the current row", () => {
    expect(moveCaret(sel(21, 21), "Home", OPTS)).toEqual(sel(16, 16));
  });

  it("End goes one past the last byte of the current row", () => {
    // Row starts at 16; End lands at rowStart + columns = 32 (= start of next row)
    expect(moveCaret(sel(21, 21), "End", OPTS)).toEqual(sel(32, 32));
  });

  it("End on the tail row clamps to past-end (length)", () => {
    expect(moveCaret(sel(99, 99), "End", { ...OPTS, length: 100 })).toEqual(
      sel(100, 100),
    );
  });

  it("Ctrl+Home jumps to offset 0", () => {
    expect(moveCaret(sel(50, 50), "Home", { ...OPTS, ctrl: true })).toEqual(
      sel(0, 0),
    );
  });

  it("Ctrl+End jumps to past-end (length)", () => {
    expect(moveCaret(sel(5, 5), "End", { ...OPTS, ctrl: true })).toEqual(
      sel(100, 100),
    );
  });

  it("PageDown jumps visibleRows rows", () => {
    expect(moveCaret(sel(5, 5), "PageDown", OPTS)).toEqual(sel(69, 69));
  });

  it("PageUp jumps visibleRows rows back, clamping at 0", () => {
    expect(moveCaret(sel(69, 69), "PageUp", OPTS)).toEqual(sel(5, 5));
    expect(moveCaret(sel(10, 10), "PageUp", OPTS)).toEqual(sel(0, 0));
  });
});

describe("moveCaret - selection extension (shift)", () => {
  it("shift preserves the anchor when arrow keys move the caret", () => {
    expect(
      moveCaret(sel(10, 10), "ArrowRight", { ...OPTS, shift: true }),
    ).toEqual(sel(10, 11));
  });

  it("shift+arrow can pivot the selection through the anchor", () => {
    const start = sel(10, 12);
    const left = moveCaret(start, "ArrowLeft", { ...OPTS, shift: true })!;
    expect(left).toEqual(sel(10, 11));
    const leftAgain = moveCaret(left, "ArrowLeft", { ...OPTS, shift: true })!;
    expect(leftAgain).toEqual(sel(10, 10));
    const leftAgain2 = moveCaret(leftAgain, "ArrowLeft", {
      ...OPTS,
      shift: true,
    })!;
    expect(leftAgain2).toEqual(sel(10, 9));
  });
});

describe("moveCaret - column switching", () => {
  it("Tab toggles between hex and ASCII columns", () => {
    expect(moveCaret(sel(5, 5, "hex"), "Tab", OPTS)).toEqual(
      sel(5, 5, "ascii"),
    );
    expect(moveCaret(sel(5, 5, "ascii"), "Tab", OPTS)).toEqual(
      sel(5, 5, "hex"),
    );
  });

  it("Tab is allowed on an empty buffer", () => {
    expect(moveCaret(sel(0, 0), "Tab", { ...OPTS, length: 0 })).toEqual(
      sel(0, 0, "ascii"),
    );
  });
});

describe("moveCaret - empty buffer", () => {
  it("ArrowRight is a no-op (caret is already at length=0)", () => {
    expect(moveCaret(sel(0, 0), "ArrowRight", { ...OPTS, length: 0 })).toEqual(
      sel(0, 0),
    );
  });
});

describe("moveCaret - non-navigation keys", () => {
  it("returns null for unhandled keys", () => {
    expect(moveCaret(sel(5, 5), "a", OPTS)).toBeNull();
    expect(moveCaret(sel(5, 5), "Enter", OPTS)).toBeNull();
    expect(moveCaret(sel(5, 5), "Escape", OPTS)).toBeNull();
  });
});
