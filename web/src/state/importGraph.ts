import { parse as parseYaml } from "yaml";

import type { VFS } from "../vfs/types";
import { KSY_EXTENSION, ksyPathToImportName } from "./project";

/** A KSY file resolved as part of an import-graph walk. The `importName`
 *  is the slash-separated path without the `.ksy` extension. */
export interface ResolvedKsy {
  importName: string;
  source: string;
}

const DEFAULT_IMPORT_PATHS: readonly string[] = ["."];

export async function collectImportGraph(
  rootKsyPath: string,
  vfs: VFS,
  importPaths: readonly string[] = DEFAULT_IMPORT_PATHS,
): Promise<ResolvedKsy[]> {
  const collected = new Map<string, string>();
  const rootName = ksyPathToImportName(rootKsyPath);
  await walk("", rootName, vfs, importPaths, collected);
  return Array.from(collected, ([importName, source]) => ({
    importName,
    source,
  }));
}

async function walk(
  from: string,
  to: string,
  vfs: VFS,
  importPaths: readonly string[],
  collected: Map<string, string>,
): Promise<void> {
  const r = await resolveImport(from, to, vfs, importPaths);
  if (!r) return;
  if (collected.has(r.importName)) return;
  collected.set(r.importName, r.source);
  for (const imp of parseKsyImports(r.source)) {
    await walk(r.importName, imp, vfs, importPaths, collected);
  }
}

export async function resolveImport(
  from: string,
  to: string,
  vfs: VFS,
  importPaths: readonly string[] = DEFAULT_IMPORT_PATHS,
): Promise<ResolvedKsy | null> {
  let basename = to;
  const isAbsolute = to.startsWith("/");
  if (isAbsolute) {
    basename = to.slice(1);
  } else if (from !== "") {
    basename = goPathJoin(goPathDir(from), to);
  }

  const candidates: string[] = [];
  if (isAbsolute) {
    for (const dir of importPaths) {
      const full = goPathJoin(dir, basename);
      candidates.push(full + KSY_EXTENSION, full);
    }
  } else {
    candidates.push(basename + KSY_EXTENSION, basename);
  }

  for (const name of candidates) {
    const vfsPath = "/" + name;
    if (!(await vfs.exists(vfsPath))) continue;
    const source = await vfs.readText(vfsPath);
    const importName = name.endsWith(KSY_EXTENSION)
      ? name.slice(0, -KSY_EXTENSION.length)
      : name;
    return { importName, source };
  }
  return null;
}

export function parseKsyImports(source: string): string[] {
  let doc: unknown;
  try {
    doc = parseYaml(source);
  } catch {
    return [];
  }
  if (!doc || typeof doc !== "object") return [];
  const meta = (doc as Record<string, unknown>)["meta"];
  if (!meta || typeof meta !== "object") return [];
  const imports = (meta as Record<string, unknown>)["imports"];
  if (!Array.isArray(imports)) return [];
  return imports.filter((x): x is string => typeof x === "string");
}

export function goPathClean(p: string): string {
  if (p === "") return ".";
  const rooted = p.startsWith("/");
  const parts: string[] = [];
  for (const part of p.split("/")) {
    if (part === "" || part === ".") continue;
    if (part === "..") {
      if (parts.length > 0 && parts[parts.length - 1] !== "..") {
        parts.pop();
      } else if (!rooted) {
        parts.push("..");
      }
      continue;
    }
    parts.push(part);
  }
  let out = parts.join("/");
  if (rooted) out = "/" + out;
  if (out === "") return rooted ? "/" : ".";
  return out;
}

export function goPathDir(p: string): string {
  const i = p.lastIndexOf("/");
  if (i < 0) return ".";
  return goPathClean(p.substring(0, i + 1));
}

export function goPathJoin(...parts: string[]): string {
  const nonEmpty = parts.filter((p) => p !== "");
  if (nonEmpty.length === 0) return "";
  return goPathClean(nonEmpty.join("/"));
}
