export type Column = "hex" | "ascii";

export interface SelectionState {
  anchor: number;
  caret: number;
  column: Column;
}

/**
 * Inclusive byte range covered by the selection, or `null` if the
 * selection is collapsed (anchor === caret). Unlike the caret itself,
 * range endpoints are never the after-end position - they point at actual
 * bytes.
 */
export function selectionRange(s: SelectionState): [number, number] | null {
  if (s.anchor === s.caret) return null;
  return s.anchor < s.caret ? [s.anchor, s.caret - 1] : [s.caret, s.anchor - 1];
}

/** Whether `offset` falls inside the (inclusive) selection range. */
export function isInSelection(offset: number, s: SelectionState): boolean {
  const r = selectionRange(s);
  if (!r) return false;
  return offset >= r[0] && offset <= r[1];
}

export interface MoveOpts {
  length: number;
  columns: number;
  /** Number of fully visible rows - used for PageUp/PageDown. */
  visibleRows: number;
  ctrl: boolean;
  shift: boolean;
}

/**
 * Compute the next selection state in response to a key press. Returns
 * `null` if the key isn't a navigation key (caller should let it bubble).
 *
 * The shift modifier preserves `anchor` (extending selection); without
 * shift, the selection collapses to the new caret position.
 *
 * Caret values range over [0, length] - the phantom past-end position is
 * always reachable so the user can append.
 */
export function moveCaret(
  current: SelectionState,
  key: string,
  opts: MoveOpts,
): SelectionState | null {
  const { length, columns, visibleRows, ctrl, shift } = opts;

  if (key === "Tab") {
    return {
      anchor: current.anchor,
      caret: current.caret,
      column: current.column === "hex" ? "ascii" : "hex",
    };
  }

  const maxOffset = length;
  const rowStart = current.caret - (current.caret % columns);
  let next = current.caret;

  switch (key) {
    case "ArrowLeft":
      next = Math.max(0, current.caret - 1);
      break;
    case "ArrowRight":
      next = Math.min(maxOffset, current.caret + 1);
      break;
    case "ArrowUp": {
      const col = current.caret % columns;
      next = Math.max(col, current.caret - columns);
      break;
    }
    case "ArrowDown":
      next = Math.min(maxOffset, current.caret + columns);
      break;
    case "Home":
      next = ctrl ? 0 : rowStart;
      break;
    case "End":
      next = ctrl ? maxOffset : Math.min(maxOffset, rowStart + columns);
      break;
    case "PageUp":
      next = Math.max(0, current.caret - visibleRows * columns);
      break;
    case "PageDown":
      next = Math.min(maxOffset, current.caret + visibleRows * columns);
      break;
    default:
      return null;
  }

  return {
    anchor: shift ? current.anchor : next,
    caret: next,
    column: current.column,
  };
}
