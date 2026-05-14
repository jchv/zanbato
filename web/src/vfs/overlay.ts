import { InMemoryVFS } from "./memory";
import {
  type VFS,
  type VFSChange,
  type VFSEntry,
  type VFSListener,
  normalizePath,
} from "./types";

export class OverlayVFS implements VFS {
  private overlay: InMemoryVFS;
  private tombstones = new Set<string>();
  private listeners = new Set<VFSListener>();

  constructor(
    private base: VFS,
    overlay?: InMemoryVFS,
  ) {
    this.overlay = overlay ?? new InMemoryVFS();
    this.overlay.subscribe((change) => this.fire(change));
  }

  /** The underlying base VFS this overlay is wrapping. */
  get baseVfs(): VFS {
    return this.base;
  }

  /** The overlay VFS */
  get overlayVfs(): InMemoryVFS {
    return this.overlay;
  }

  get tombstoneList(): string[] {
    return Array.from(this.tombstones);
  }

  async list(path: string): Promise<VFSEntry[]> {
    const seen = new Map<string, VFSEntry>();
    for (const entry of await this.base.list(path)) {
      if (this.isTombstoned(entry.path)) continue;
      seen.set(entry.path, entry);
    }
    // Overlay entries override base entries with the same path.
    for (const entry of await this.overlay.list(path)) {
      seen.set(entry.path, entry);
    }
    return Array.from(seen.values()).sort(compareEntries);
  }

  async read(path: string): Promise<Uint8Array> {
    const norm = normalizePath(path);
    if (await this.overlayHasFile(norm)) {
      return this.overlay.read(norm);
    }
    if (this.isTombstoned(norm)) {
      throw new Error(`File not found: ${norm}`);
    }
    return this.base.read(norm);
  }

  async readText(path: string): Promise<string> {
    return new TextDecoder().decode(await this.read(path));
  }

  async write(path: string, data: Uint8Array | string): Promise<void> {
    const norm = normalizePath(path);
    this.tombstones.delete(norm);
    await this.overlay.write(norm, data);
  }

  async mkdir(path: string): Promise<void> {
    const norm = normalizePath(path);
    this.tombstones.delete(norm);
    await this.overlay.mkdir(norm);
  }

  async delete(path: string): Promise<void> {
    const norm = normalizePath(path);
    if (norm === "/") {
      // Wipe everything: clear overlay + tombstone the root.
      await this.overlay.delete("/");
      this.tombstones.clear();
      this.tombstones.add("/");
      this.fire({ path: "/", kind: "delete" });
      return;
    }
    let changed = false;
    if (await this.overlay.exists(norm)) {
      await this.overlay.delete(norm); // fires its own event
      changed = true;
    }
    if ((await this.base.exists(norm)) && !this.isTombstoned(norm)) {
      this.tombstones.add(norm);
      changed = true;
    }
    if (changed && !(await this.overlay.exists(norm))) {
      this.fire({ path: norm, kind: "delete" });
    }
  }

  async exists(path: string): Promise<boolean> {
    const norm = normalizePath(path);
    if (await this.overlay.exists(norm)) return true;
    if (this.isTombstoned(norm)) return false;
    return this.base.exists(norm);
  }

  subscribe(listener: VFSListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  private isTombstoned(path: string): boolean {
    if (this.tombstones.has(path)) return true;
    for (const t of this.tombstones) {
      if (path.startsWith(t + "/")) return true;
    }
    return false;
  }

  private async overlayHasFile(path: string): Promise<boolean> {
    try {
      await this.overlay.read(path);
      return true;
    } catch {
      return false;
    }
  }

  private fire(change: VFSChange): void {
    for (const l of this.listeners) l(change);
  }
}

function compareEntries(a: VFSEntry, b: VFSEntry): number {
  if (a.type !== b.type) return a.type === "directory" ? -1 : 1;
  return a.name.localeCompare(b.name);
}
