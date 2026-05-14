interface Piece {
  readonly buffer: Uint8Array;
  readonly offset: number;
  readonly length: number;
}

export interface PieceTableChange {
  offset: number;
  removedLength: number;
  insertedLength: number;
  version: number;
}

export type PieceTableListener = (event: PieceTableChange) => void;

export interface PieceTableChunk {
  readonly buffer: Uint8Array;
  readonly offset: number;
  readonly length: number;
}

export class PieceTable {
  private pieces: Piece[];
  private _length: number;
  private _version = 0;
  private undoStack: Piece[][] = [];
  private redoStack: Piece[][] = [];
  private listeners = new Set<PieceTableListener>();

  constructor(initial: Uint8Array = new Uint8Array(0)) {
    if (initial.length > 0) {
      // Copy so the caller can't mutate our bytes after construction.
      const buf = initial.slice();
      this.pieces = [{ buffer: buf, offset: 0, length: buf.length }];
      this._length = buf.length;
    } else {
      this.pieces = [];
      this._length = 0;
    }
  }

  get length(): number {
    return this._length;
  }
  get version(): number {
    return this._version;
  }
  get canUndo(): boolean {
    return this.undoStack.length > 0;
  }
  get canRedo(): boolean {
    return this.redoStack.length > 0;
  }

  /**
   * Locate the piece containing the byte at `offset`. `offset === length`
   * is allowed and returns the position immediately past the last piece.
   */
  private locate(offset: number): { index: number; localOffset: number } {
    if (offset < 0 || offset > this._length) {
      throw new RangeError(
        `offset ${offset} out of range [0, ${this._length}]`,
      );
    }
    let acc = 0;
    for (let i = 0; i < this.pieces.length; i++) {
      const p = this.pieces[i]!;
      if (offset < acc + p.length) {
        return { index: i, localOffset: offset - acc };
      }
      acc += p.length;
    }
    return { index: this.pieces.length, localOffset: 0 };
  }

  readByte(offset: number): number {
    if (offset < 0 || offset >= this._length) {
      throw new RangeError(
        `offset ${offset} out of range [0, ${this._length})`,
      );
    }
    const { index, localOffset } = this.locate(offset);
    const p = this.pieces[index]!;
    return p.buffer[p.offset + localOffset]!;
  }

  /** Allocate and return `length` bytes starting at `offset`. */
  read(offset: number, length: number): Uint8Array {
    if (length < 0) throw new RangeError(`negative length ${length}`);
    if (offset < 0 || offset + length > this._length) {
      throw new RangeError(
        `range [${offset}, ${offset + length}) out of buffer [0, ${this._length})`,
      );
    }
    const out = new Uint8Array(length);
    if (length === 0) return out;
    let written = 0;
    for (const chunk of this.chunks(offset, length)) {
      out.set(
        chunk.buffer.subarray(chunk.offset, chunk.offset + chunk.length),
        written,
      );
      written += chunk.length;
    }
    return out;
  }

  /**
   * Yield zero-copy views over the bytes in [offset, offset+length). The
   * caller MUST NOT mutate the yielded `buffer` - it is shared with the
   * piece table's internal state.
   */
  *chunks(offset: number, length: number): Generator<PieceTableChunk> {
    if (length === 0) return;
    if (length < 0) throw new RangeError(`negative length ${length}`);
    if (offset < 0 || offset + length > this._length) {
      throw new RangeError(
        `range [${offset}, ${offset + length}) out of buffer [0, ${this._length})`,
      );
    }
    let { index, localOffset } = this.locate(offset);
    let remaining = length;
    while (remaining > 0) {
      const p = this.pieces[index]!;
      const available = p.length - localOffset;
      const take = available < remaining ? available : remaining;
      yield { buffer: p.buffer, offset: p.offset + localOffset, length: take };
      remaining -= take;
      index++;
      localOffset = 0;
    }
  }

  /** Materialize the full buffer as a fresh Uint8Array. */
  toBytes(): Uint8Array {
    const out = new Uint8Array(this._length);
    let pos = 0;
    for (const p of this.pieces) {
      out.set(p.buffer.subarray(p.offset, p.offset + p.length), pos);
      pos += p.length;
    }
    return out;
  }

  insert(offset: number, bytes: Uint8Array): void {
    if (offset < 0 || offset > this._length) {
      throw new RangeError(
        `offset ${offset} out of range [0, ${this._length}]`,
      );
    }
    if (bytes.length === 0) return;
    this.snapshot();
    this.insertInternal(offset, bytes);
    this.fire({
      offset,
      removedLength: 0,
      insertedLength: bytes.length,
      version: ++this._version,
    });
  }

  delete(offset: number, count: number): void {
    if (count < 0) throw new RangeError(`negative count ${count}`);
    if (offset < 0 || offset + count > this._length) {
      throw new RangeError(
        `range [${offset}, ${offset + count}) out of buffer [0, ${this._length})`,
      );
    }
    if (count === 0) return;
    this.snapshot();
    this.deleteInternal(offset, count);
    this.fire({
      offset,
      removedLength: count,
      insertedLength: 0,
      version: ++this._version,
    });
  }

  /**
   * Replace bytes starting at `offset` with `bytes`. If
   * `offset + bytes.length` exceeds the current length, the buffer grows.
   * Emits a single change event (one undo step) regardless of whether the
   * operation shortens, lengthens, or matches the existing region.
   */
  overwrite(offset: number, bytes: Uint8Array): void {
    if (offset < 0 || offset > this._length) {
      throw new RangeError(
        `offset ${offset} out of range [0, ${this._length}]`,
      );
    }
    if (bytes.length === 0) return;
    const toDelete = Math.min(bytes.length, this._length - offset);
    this.snapshot();
    if (toDelete > 0) this.deleteInternal(offset, toDelete);
    this.insertInternal(offset, bytes);
    this.fire({
      offset,
      removedLength: toDelete,
      insertedLength: bytes.length,
      version: ++this._version,
    });
  }

  undo(): boolean {
    if (this.undoStack.length === 0) return false;
    const prev = this.undoStack.pop()!;
    this.redoStack.push(this.pieces);
    const oldLength = this._length;
    this.pieces = prev;
    this._length = sumLengths(prev);
    this.fire({
      offset: 0,
      removedLength: oldLength,
      insertedLength: this._length,
      version: ++this._version,
    });
    return true;
  }

  redo(): boolean {
    if (this.redoStack.length === 0) return false;
    const next = this.redoStack.pop()!;
    this.undoStack.push(this.pieces);
    const oldLength = this._length;
    this.pieces = next;
    this._length = sumLengths(next);
    this.fire({
      offset: 0,
      removedLength: oldLength,
      insertedLength: this._length,
      version: ++this._version,
    });
    return true;
  }

  subscribe(listener: PieceTableListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  private insertInternal(offset: number, bytes: Uint8Array): void {
    // Own the bytes. Each insert allocates a fresh buffer; pieces never
    // share a mutable backing store with anything outside the table.
    const buf = bytes.slice();
    const newPiece: Piece = { buffer: buf, offset: 0, length: buf.length };
    if (this.pieces.length === 0) {
      this.pieces = [newPiece];
    } else {
      const { index, localOffset } = this.locate(offset);
      if (localOffset === 0) {
        this.pieces.splice(index, 0, newPiece);
      } else {
        const p = this.pieces[index]!;
        const left: Piece = {
          buffer: p.buffer,
          offset: p.offset,
          length: localOffset,
        };
        const right: Piece = {
          buffer: p.buffer,
          offset: p.offset + localOffset,
          length: p.length - localOffset,
        };
        this.pieces.splice(index, 1, left, newPiece, right);
      }
    }
    this._length += bytes.length;
  }

  private deleteInternal(offset: number, count: number): void {
    const { index: startIdx, localOffset: startLocal } = this.locate(offset);
    const replacement: Piece[] = [];
    let cursor = startIdx;
    let localOffset = startLocal;
    let remaining = count;
    while (remaining > 0 && cursor < this.pieces.length) {
      const p = this.pieces[cursor]!;
      const available = p.length - localOffset;
      // Save the prefix of the start piece (if any) - but only once, on
      // the first iteration. On later pieces, localOffset is always 0 and
      // we skip this branch naturally.
      if (localOffset > 0) {
        replacement.push({
          buffer: p.buffer,
          offset: p.offset,
          length: localOffset,
        });
      }
      if (remaining >= available) {
        remaining -= available;
        cursor++;
        localOffset = 0;
      } else {
        // Partial: keep the suffix after the deleted span.
        replacement.push({
          buffer: p.buffer,
          offset: p.offset + localOffset + remaining,
          length: available - remaining,
        });
        remaining = 0;
        cursor++;
      }
    }
    this.pieces.splice(startIdx, cursor - startIdx, ...replacement);
    this._length -= count;
  }

  private snapshot(): void {
    // Shallow copy of the pieces array. Pieces themselves are immutable so
    // they can be safely shared between snapshots.
    this.undoStack.push(this.pieces.slice());
    this.redoStack.length = 0;
  }

  private fire(event: PieceTableChange): void {
    for (const l of this.listeners) l(event);
  }
}

function sumLengths(pieces: readonly Piece[]): number {
  let total = 0;
  for (const p of pieces) total += p.length;
  return total;
}
