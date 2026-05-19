import { type VFS, basename, normalizePath } from "../vfs/types";

export const KSY_EXTENSION = ".ksy";

export interface Project {
  vfs: VFS;
  currentKsyPath: string | null;
  currentBinaryPath: string | null;
}

export function ksyPathToImportName(path: string): string {
  let p = normalizePath(path);
  if (p.startsWith("/")) p = p.slice(1);
  if (p.endsWith(KSY_EXTENSION)) p = p.slice(0, -KSY_EXTENSION.length);
  return p;
}

export function ksyDisplayName(path: string): string {
  const b = basename(path);
  return b.endsWith(KSY_EXTENSION) ? b.slice(0, -KSY_EXTENSION.length) : b;
}
