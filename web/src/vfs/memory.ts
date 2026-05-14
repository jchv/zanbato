import {
  type VFS,
  type VFSChange,
  type VFSEntry,
  type VFSListener,
  basename,
  dirname,
  normalizePath,
} from "./types";

export class InMemoryVFS implements VFS {
  private files = new Map<string, Uint8Array>();
  private emptyDirs = new Set<string>();
  private listeners = new Set<VFSListener>();

  static fromSeed(seed: Record<string, Uint8Array | string>): InMemoryVFS {
    const vfs = new InMemoryVFS();
    for (const [path, data] of Object.entries(seed)) {
      vfs.files.set(normalizePath(path), toBytes(data).slice());
    }
    return vfs;
  }

  async list(path: string): Promise<VFSEntry[]> {
    const dir = normalizePath(path);
    const prefix = dir === "/" ? "/" : dir + "/";
    const seen = new Map<string, VFSEntry>();

    // Files (and the directories implied by their paths).
    for (const filePath of this.files.keys()) {
      if (!filePath.startsWith(prefix)) continue;
      const rest = filePath.slice(prefix.length);
      const slash = rest.indexOf("/");
      if (slash === -1) {
        const childPath = prefix + rest;
        seen.set(childPath, {
          path: childPath,
          name: rest,
          type: "file",
        });
      } else {
        const dirName = rest.slice(0, slash);
        const childPath = prefix + dirName;
        if (!seen.has(childPath)) {
          seen.set(childPath, {
            path: childPath,
            name: dirName,
            type: "directory",
          });
        }
      }
    }

    for (const emptyDir of this.emptyDirs) {
      if (!emptyDir.startsWith(prefix)) continue;
      const rest = emptyDir.slice(prefix.length);
      if (rest.length === 0) continue;
      const slash = rest.indexOf("/");
      const dirName = slash === -1 ? rest : rest.slice(0, slash);
      const childPath = prefix + dirName;
      if (!seen.has(childPath)) {
        seen.set(childPath, {
          path: childPath,
          name: dirName,
          type: "directory",
        });
      }
    }

    return Array.from(seen.values()).sort(compareEntries);
  }

  async read(path: string): Promise<Uint8Array> {
    const norm = normalizePath(path);
    const data = this.files.get(norm);
    if (!data) throw new Error(`File not found: ${norm}`);
    return data;
  }

  async readText(path: string): Promise<string> {
    return new TextDecoder().decode(await this.read(path));
  }

  async write(path: string, data: Uint8Array | string): Promise<void> {
    const norm = normalizePath(path);
    if (norm === "/") throw new Error("cannot write to /");
    this.files.set(norm, toBytes(data).slice());
    for (const emptyDir of Array.from(this.emptyDirs)) {
      if (norm.startsWith(emptyDir + "/")) {
        this.emptyDirs.delete(emptyDir);
      }
    }
    this.fire({ path: norm, kind: "write" });
  }

  async mkdir(path: string): Promise<void> {
    const norm = normalizePath(path);
    if (norm === "/") return;
    if (await this.exists(norm)) return;
    this.emptyDirs.add(norm);
    this.fire({ path: norm, kind: "write" });
  }

  async delete(path: string): Promise<void> {
    const norm = normalizePath(path);
    if (norm === "/") {
      if (this.files.size === 0 && this.emptyDirs.size === 0) return;
      this.files.clear();
      this.emptyDirs.clear();
      this.fire({ path: "/", kind: "delete" });
      return;
    }
    let deleted = false;
    const prefix = norm + "/";
    for (const filePath of Array.from(this.files.keys())) {
      if (filePath === norm || filePath.startsWith(prefix)) {
        this.files.delete(filePath);
        deleted = true;
      }
    }
    for (const emptyDir of Array.from(this.emptyDirs)) {
      if (emptyDir === norm || emptyDir.startsWith(prefix)) {
        this.emptyDirs.delete(emptyDir);
        deleted = true;
      }
    }
    if (deleted) this.fire({ path: norm, kind: "delete" });
  }

  async exists(path: string): Promise<boolean> {
    const norm = normalizePath(path);
    if (norm === "/") return true;
    if (this.files.has(norm)) return true;
    if (this.emptyDirs.has(norm)) return true;
    const prefix = norm + "/";
    for (const filePath of this.files.keys()) {
      if (filePath.startsWith(prefix)) return true;
    }
    for (const emptyDir of this.emptyDirs) {
      if (emptyDir.startsWith(prefix)) return true;
    }
    return false;
  }

  subscribe(listener: VFSListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  private fire(change: VFSChange): void {
    for (const l of this.listeners) l(change);
  }
}

function toBytes(data: Uint8Array | string): Uint8Array {
  return typeof data === "string" ? new TextEncoder().encode(data) : data;
}

function compareEntries(a: VFSEntry, b: VFSEntry): number {
  if (a.type !== b.type) {
    return a.type === "directory" ? -1 : 1;
  }
  return a.name.localeCompare(b.name);
}

export { basename, dirname, normalizePath };
