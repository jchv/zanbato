export type VFSEntryType = "file" | "directory";

export interface VFSEntry {
  path: string;
  name: string;
  type: VFSEntryType;
}

export type VFSChangeKind = "write" | "delete";

export interface VFSChange {
  path: string;
  kind: VFSChangeKind;
}

export type VFSListener = (change: VFSChange) => void;

export interface VFS {
  list(path: string): Promise<VFSEntry[]>;
  read(path: string): Promise<Uint8Array>;
  readText(path: string): Promise<string>;
  write(path: string, data: Uint8Array | string): Promise<void>;
  mkdir(path: string): Promise<void>;
  delete(path: string): Promise<void>;
  exists(path: string): Promise<boolean>;
  subscribe(listener: VFSListener): () => void;
}

export function normalizePath(path: string): string {
  if (!path.startsWith("/")) path = "/" + path;
  while (path.length > 1 && path.endsWith("/")) {
    path = path.slice(0, -1);
  }
  return path;
}

export function basename(path: string): string {
  const norm = normalizePath(path);
  if (norm === "/") return "";
  const idx = norm.lastIndexOf("/");
  return norm.slice(idx + 1);
}

export function dirname(path: string): string {
  const norm = normalizePath(path);
  if (norm === "/") return "/";
  const idx = norm.lastIndexOf("/");
  return idx <= 0 ? "/" : norm.slice(0, idx);
}
