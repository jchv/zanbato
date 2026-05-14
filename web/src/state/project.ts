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

export async function collectKsyFiles(
  vfs: VFS,
): Promise<Array<{ path: string; source: string }>> {
  const out: Array<{ path: string; source: string }> = [];
  await walk(vfs, "/", out);
  return out;
}

async function walk(
  vfs: VFS,
  dir: string,
  out: Array<{ path: string; source: string }>,
): Promise<void> {
  const entries = await vfs.list(dir);
  for (const entry of entries) {
    if (entry.type === "directory") {
      await walk(vfs, entry.path, out);
    } else if (entry.path.endsWith(KSY_EXTENSION)) {
      const source = await vfs.readText(entry.path);
      out.push({ path: entry.path, source });
    }
  }
}
