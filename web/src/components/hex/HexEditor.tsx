import {
  memo,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import type { PieceTable } from "../../buffer/pieceTable";
import {
  byteToAscii,
  byteToHex,
  formatOffset,
  offsetWidthFor,
} from "./byteFormat";
import {
  type EditContext,
  type EditResult,
  type Nibble,
  isPrintableAscii,
  backspace as opBackspace,
  deleteForward as opDeleteForward,
  typeAscii as opTypeAscii,
  typeHexDigit as opTypeHexDigit,
  parseHexDigit,
} from "./editOps";
import "./hex.css";
import {
  type Column,
  type SelectionState,
  isInSelection,
  moveCaret,
  selectionRange,
} from "./selection";

const DEFAULT_COLUMNS = 16;
const DEFAULT_ROW_HEIGHT = 18;
const OVERSCAN = 6;

export type SelectionSource = "pointer" | "keyboard";

export interface HexEditorProps {
  buffer: PieceTable;
  selection: SelectionState;
  onSelectionChange: (s: SelectionState, source: SelectionSource) => void;
  /** Inclusive byte range to overlay as a tree-driven highlight. */
  highlight?: [number, number] | null;
  columns?: number;
  rowHeight?: number;
  onFileLoad?: (filename: string, bytes: Uint8Array) => void;
}

export function HexEditor({
  buffer,
  selection,
  onSelectionChange,
  highlight = null,
  columns = DEFAULT_COLUMNS,
  rowHeight = DEFAULT_ROW_HEIGHT,
  onFileLoad,
}: HexEditorProps) {
  // Force re-render on buffer change (edits from anywhere bump version).
  const [, setVersion] = useState(buffer.version);
  useEffect(() => {
    return buffer.subscribe(() => setVersion(buffer.version));
  }, [buffer]);

  const containerRef = useRef<HTMLDivElement>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportHeight, setViewportHeight] = useState(0);

  useLayoutEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    setViewportHeight(el.clientHeight);
    const ro = new ResizeObserver((entries) => {
      for (const e of entries) setViewportHeight(e.contentRect.height);
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const length = buffer.length;
  const totalRows = Math.max(1, Math.ceil((length + 1) / columns));
  const offsetWidth = useMemo(() => offsetWidthFor(length), [length]);

  const firstVisible = Math.floor(scrollTop / rowHeight);
  const visibleCount = Math.max(1, Math.ceil(viewportHeight / rowHeight));
  const start = Math.max(0, firstVisible - OVERSCAN);
  const end = Math.min(totalRows, firstVisible + visibleCount + OVERSCAN);

  const [nibble, setNibble] = useState<Nibble>("high");
  const [insertMode, setInsertMode] = useState<boolean>(false);

  // If the buffer shrinks past the caret, clamp via the controlled
  // callback. This propagates through the workspace so the tree panel
  // also re-syncs.
  useEffect(() => {
    if (selection.caret > length || selection.anchor > length) {
      onSelectionChange(
        {
          anchor: Math.min(selection.anchor, length),
          caret: Math.min(selection.caret, length),
          column: selection.column,
        },
        "keyboard",
      );
    }
  }, [
    length,
    selection.anchor,
    selection.caret,
    selection.column,
    onSelectionChange,
  ]);

  // Auto-scroll the caret into view on selection change. Reading the
  // viewport from refs avoids a feedback loop with scrollTop.
  const scrollTopRef = useRef(scrollTop);
  const viewportHeightRef = useRef(viewportHeight);
  useLayoutEffect(() => {
    scrollTopRef.current = scrollTop;
    viewportHeightRef.current = viewportHeight;
  });

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const caretRow = Math.floor(selection.caret / columns);
    const topOfRow = caretRow * rowHeight;
    const bottomOfRow = topOfRow + rowHeight;
    const viewTop = scrollTopRef.current;
    const viewBottom = viewTop + viewportHeightRef.current;
    if (topOfRow < viewTop) {
      el.scrollTop = topOfRow;
    } else if (bottomOfRow > viewBottom) {
      el.scrollTop = bottomOfRow - viewportHeightRef.current;
    }
  }, [selection.caret, columns, rowHeight]);

  // Mouse handlers
  const draggingRef = useRef(false);
  const pointerIdRef = useRef<number | null>(null);

  const byteAtPoint = (
    x: number,
    y: number,
  ): { offset: number; column: Column } | null => {
    const el = document.elementFromPoint(x, y);
    if (!el) return null;
    const byteEl = (el as HTMLElement).closest<HTMLElement>("[data-offset]");
    if (!byteEl) return null;
    const offset = parseInt(byteEl.dataset["offset"] ?? "", 10);
    const column = byteEl.dataset["column"] as Column | undefined;
    if (!Number.isFinite(offset) || (column !== "hex" && column !== "ascii"))
      return null;
    return { offset, column };
  };

  const onPointerDown = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      const hit = byteAtPoint(e.clientX, e.clientY);
      if (!hit) return;
      const target = e.currentTarget;
      target.setPointerCapture(e.pointerId);
      pointerIdRef.current = e.pointerId;
      draggingRef.current = true;
      target.focus();
      setNibble("high");
      onSelectionChange(
        e.shiftKey
          ? { anchor: selection.anchor, caret: hit.offset, column: hit.column }
          : { anchor: hit.offset, caret: hit.offset, column: hit.column },
        "pointer",
      );
    },
    [selection.anchor, onSelectionChange],
  );

  const onPointerMove = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      if (!draggingRef.current) return;
      const hit = byteAtPoint(e.clientX, e.clientY);
      if (!hit) return;
      if (selection.caret === hit.offset && selection.column === hit.column)
        return;
      onSelectionChange(
        { anchor: selection.anchor, caret: hit.offset, column: hit.column },
        "pointer",
      );
    },
    [selection.anchor, selection.caret, selection.column, onSelectionChange],
  );

  const onPointerUp = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    draggingRef.current = false;
    if (pointerIdRef.current !== null) {
      e.currentTarget.releasePointerCapture(pointerIdRef.current);
      pointerIdRef.current = null;
    }
  }, []);

  // Edits
  const applyEdit = useCallback(
    (result: EditResult) => {
      for (const op of result.ops) {
        switch (op.kind) {
          case "insert":
            buffer.insert(op.offset, op.data);
            break;
          case "overwrite":
            buffer.overwrite(op.offset, op.data);
            break;
          case "delete":
            buffer.delete(op.offset, op.count);
            break;
        }
      }
      onSelectionChange(
        {
          anchor: result.newCaret,
          caret: result.newCaret,
          column: selection.column,
        },
        "keyboard",
      );
      setNibble(result.newNibble);
    },
    [buffer, selection.column, onSelectionChange],
  );

  // Keyboard handler
  const onKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLDivElement>) => {
      const nav = moveCaret(selection, e.key, {
        length,
        columns,
        visibleRows: visibleCount,
        ctrl: e.ctrlKey || e.metaKey,
        shift: e.shiftKey,
      });
      if (nav) {
        e.preventDefault();
        onSelectionChange(nav, "keyboard");
        setNibble("high");
        return;
      }

      if (e.key === "Insert") {
        e.preventDefault();
        setInsertMode((m) => !m);
        return;
      }

      const range = selectionRange(selection);
      const caretByte =
        selection.caret < length ? buffer.readByte(selection.caret) : undefined;
      const ctx: EditContext = {
        caret: selection.caret,
        length,
        byteAtCaret: caretByte,
        nibble,
        insertMode,
        selectionRange: range,
      };

      if (e.key === "Backspace") {
        e.preventDefault();
        applyEdit(opBackspace(ctx));
        return;
      }
      if (e.key === "Delete") {
        e.preventDefault();
        applyEdit(opDeleteForward(ctx));
        return;
      }

      if (selection.column === "hex") {
        const digit = parseHexDigit(e.key);
        if (digit !== null) {
          e.preventDefault();
          applyEdit(opTypeHexDigit(digit, ctx));
        }
        return;
      }

      if (isPrintableAscii(e.key)) {
        e.preventDefault();
        applyEdit(opTypeAscii(e.key.charCodeAt(0), ctx));
      }
    },
    [
      selection,
      length,
      columns,
      visibleCount,
      nibble,
      insertMode,
      buffer,
      applyEdit,
      onSelectionChange,
    ],
  );

  // Drag-and-drop
  const dragDepthRef = useRef(0);
  const [isDragOver, setIsDragOver] = useState(false);

  const hasFiles = (e: React.DragEvent): boolean => {
    const types = e.dataTransfer.types;
    for (let i = 0; i < types.length; i++) {
      if (types[i] === "Files") return true;
    }
    return false;
  };

  const onDragEnter = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    dragDepthRef.current += 1;
    setIsDragOver(true);
  }, []);

  const onDragOver = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
  }, []);

  const onDragLeave = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    if (!hasFiles(e)) return;
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
    if (dragDepthRef.current === 0) setIsDragOver(false);
  }, []);

  const onDrop = useCallback(
    async (e: React.DragEvent<HTMLDivElement>) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      dragDepthRef.current = 0;
      setIsDragOver(false);
      const file = e.dataTransfer.files[0];
      if (!file || !onFileLoad) return;
      const buf = await file.arrayBuffer();
      onFileLoad(file.name, new Uint8Array(buf));
    },
    [onFileLoad],
  );

  // Rendering
  const rows: React.ReactNode[] = [];
  for (let i = start; i < end; i++) {
    const rowStart = i * columns;
    const rowLen = Math.max(0, Math.min(columns, length - rowStart));
    const bytes =
      rowLen > 0 ? buffer.read(rowStart, rowLen) : new Uint8Array(0);
    rows.push(
      <HexRow
        key={i}
        offset={rowStart}
        offsetWidth={offsetWidth}
        bytes={bytes}
        columns={columns}
        top={i * rowHeight}
        height={rowHeight}
        selection={selection}
        bufferLength={length}
        insertMode={insertMode}
        highlight={highlight}
      />,
    );
  }

  return (
    <div
      className="hex-editor-wrap"
      onDragEnter={onDragEnter}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
    >
      <div
        ref={containerRef}
        className="hex-editor"
        tabIndex={0}
        onScroll={(e) => setScrollTop(e.currentTarget.scrollTop)}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onKeyDown={onKeyDown}
      >
        <div className="hex-sizer" style={{ height: totalRows * rowHeight }}>
          {rows}
        </div>
      </div>
      <HexStatusBar
        caret={selection.caret}
        length={length}
        insertMode={insertMode}
        column={selection.column}
      />
      {isDragOver && (
        <div className="hex-drop-overlay" aria-hidden="true">
          <div className="hex-drop-message">Drop binary file to load</div>
        </div>
      )}
    </div>
  );
}

interface HexRowProps {
  offset: number;
  offsetWidth: number;
  bytes: Uint8Array;
  columns: number;
  top: number;
  height: number;
  selection: SelectionState;
  bufferLength: number;
  insertMode: boolean;
  highlight: [number, number] | null;
}

const HexRow = memo(function HexRow({
  offset,
  offsetWidth,
  bytes,
  columns,
  top,
  height,
  selection,
  bufferLength,
  insertMode,
  highlight,
}: HexRowProps) {
  const hexChildren: React.ReactNode[] = [];
  const asciiChildren: React.ReactNode[] = [];

  for (let i = 0; i < columns; i++) {
    const byteOffset = offset + i;
    const hasData = byteOffset < bufferLength;
    const isCaret = byteOffset === selection.caret;
    const inSel = isInSelection(byteOffset, selection);
    const inHighlight =
      highlight !== null &&
      byteOffset >= highlight[0] &&
      byteOffset <= highlight[1];

    if (i > 0) hexChildren.push(" ");

    const opts = {
      hasData,
      inSel,
      isCaret,
      inHighlight,
      activeColumn: selection.column,
      insertMode,
    };

    hexChildren.push(
      <span
        key={`h${i}`}
        className={byteClasses("hex", opts)}
        data-offset={byteOffset}
        data-column="hex"
      >
        {hasData ? byteToHex(bytes[i]!) : "  "}
      </span>,
    );
    asciiChildren.push(
      <span
        key={`a${i}`}
        className={byteClasses("ascii", opts)}
        data-offset={byteOffset}
        data-column="ascii"
      >
        {hasData ? byteToAscii(bytes[i]!) : " "}
      </span>,
    );
  }

  return (
    <div className="hex-row" style={{ top, height }}>
      <span className="hex-offset">{formatOffset(offset, offsetWidth)}</span>
      <span className="hex-bytes">{hexChildren}</span>
      <span className="hex-ascii">{asciiChildren}</span>
    </div>
  );
});

interface ByteClassOpts {
  hasData: boolean;
  inSel: boolean;
  isCaret: boolean;
  inHighlight: boolean;
  activeColumn: Column;
  insertMode: boolean;
}

function byteClasses(
  column: Column,
  {
    hasData,
    inSel,
    isCaret,
    inHighlight,
    activeColumn,
    insertMode,
  }: ByteClassOpts,
): string {
  let cls = "hex-byte";
  if (!hasData) cls += " hex-byte--pad";
  if (inHighlight) cls += " hex-byte--tree-hl";
  const active = column === activeColumn;
  if (inSel) cls += active ? " hex-byte--sel" : " hex-byte--sel-inactive";
  if (isCaret) {
    if (insertMode) {
      cls += active ? " hex-byte--caret-ins" : " hex-byte--caret-ins-inactive";
    } else {
      cls += active ? " hex-byte--caret" : " hex-byte--caret-inactive";
    }
  }
  return cls;
}

interface HexStatusBarProps {
  caret: number;
  length: number;
  insertMode: boolean;
  column: Column;
}

function HexStatusBar({
  caret,
  length,
  insertMode,
  column,
}: HexStatusBarProps) {
  return (
    <div className="hex-status">
      <span>0x{caret.toString(16).toUpperCase().padStart(8, "0")}</span>
      <span className="hex-status-sep">/</span>
      <span>0x{length.toString(16).toUpperCase().padStart(8, "0")}</span>
      <span className="hex-status-mode">{insertMode ? "INS" : "OVR"}</span>
      <span className="hex-status-col">{column.toUpperCase()}</span>
    </div>
  );
}
