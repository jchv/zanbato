import { describe, expect, it } from "vitest";

import { InMemoryVFS } from "../vfs/memory";
import {
  collectImportGraph,
  goPathClean,
  goPathDir,
  goPathJoin,
  parseKsyImports,
  resolveImport,
} from "./importGraph";

describe("goPath helpers", () => {
  it("matches Go path.Clean for the cases the resolver hits", () => {
    expect(goPathClean("")).toBe(".");
    expect(goPathClean("a/b/../c")).toBe("a/c");
    expect(goPathClean("./a")).toBe("a");
    expect(goPathClean("a//b")).toBe("a/b");
    expect(goPathClean("/")).toBe("/");
    expect(goPathClean("/..")).toBe("/");
    expect(goPathClean("../a")).toBe("../a");
  });

  it("matches Go path.Dir", () => {
    expect(goPathDir("foo")).toBe(".");
    expect(goPathDir("a/b")).toBe("a");
    expect(goPathDir("/a/b")).toBe("/a");
    expect(goPathDir("/")).toBe("/");
    expect(goPathDir("")).toBe(".");
  });

  it("matches Go path.Join", () => {
    expect(goPathJoin("a", "b")).toBe("a/b");
    expect(goPathJoin("a/b", "../c")).toBe("a/c");
    expect(goPathJoin(".", "a/b")).toBe("a/b");
    expect(goPathJoin("", "a")).toBe("a");
  });
});

describe("parseKsyImports", () => {
  it("returns an empty list when meta.imports is missing", () => {
    expect(parseKsyImports("meta:\n  id: x\n")).toEqual([]);
  });

  it("extracts a flat list", () => {
    const src = [
      "meta:",
      "  id: x",
      "  imports:",
      "    - a",
      "    - b/c",
      "",
    ].join("\n");
    expect(parseKsyImports(src)).toEqual(["a", "b/c"]);
  });

  it("returns an empty list for invalid yaml", () => {
    expect(parseKsyImports(":\n:\n  x: [unbalanced")).toEqual([]);
  });

  it("filters non-string entries", () => {
    const src = ["meta:", "  imports:", "    - a", "    - 42", ""].join("\n");
    expect(parseKsyImports(src)).toEqual(["a"]);
  });
});

describe("resolveImport", () => {
  it("resolves a top-level import against the VFS root", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/main.ksy": "meta:\n  id: main\n",
    });
    const r = await resolveImport("", "main", vfs);
    expect(r).toEqual({
      importName: "main",
      source: "meta:\n  id: main\n",
    });
  });

  it("resolves a relative import against the importer's directory", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/sub/a.ksy": "meta:\n  id: a\n",
      "/sub/b.ksy": "meta:\n  id: b\n",
    });
    const r = await resolveImport("sub/a", "b", vfs);
    expect(r?.importName).toBe("sub/b");
  });

  it("returns null when the candidate is missing", async () => {
    const vfs = InMemoryVFS.fromSeed({});
    expect(await resolveImport("", "missing", vfs)).toBeNull();
  });

  it("resolves an absolute import against the VFS root by default", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/common/dos_datetime.ksy": "meta:\n  id: dos_datetime\n",
      "/filesystem/vfat.ksy":
        "meta:\n  id: vfat\n  imports:\n    - /common/dos_datetime\n",
    });
    const r = await resolveImport(
      "filesystem/vfat",
      "/common/dos_datetime",
      vfs,
    );
    expect(r?.importName).toBe("common/dos_datetime");
  });

  it("returns null for absolute imports when no import paths are configured", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/foo.ksy": "meta:\n  id: foo\n",
    });
    expect(await resolveImport("", "/foo", vfs, [])).toBeNull();
  });

  it("searches all configured import paths for absolute imports", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/vendor/lib/x.ksy": "meta:\n  id: x\n",
    });
    const r = await resolveImport("", "/lib/x", vfs, [".", "vendor"]);
    expect(r?.importName).toBe("vendor/lib/x");
  });
});

describe("collectImportGraph", () => {
  it("returns just the root for a no-imports KSY", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/main.ksy": "meta:\n  id: main\n",
    });
    const graph = await collectImportGraph("/main.ksy", vfs);
    expect(graph.map((f) => f.importName)).toEqual(["main"]);
  });

  it("walks transitive imports", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/main.ksy": "meta:\n  id: main\n  imports:\n    - a\n",
      "/a.ksy": "meta:\n  id: a\n  imports:\n    - sub/b\n",
      "/sub/b.ksy": "meta:\n  id: b\n",
    });
    const graph = await collectImportGraph("/main.ksy", vfs);
    expect(graph.map((f) => f.importName).sort()).toEqual([
      "a",
      "main",
      "sub/b",
    ]);
  });

  it("resolves sibling imports relative to the importer's directory", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/main.ksy": "meta:\n  id: main\n  imports:\n    - sub/a\n",
      "/sub/a.ksy": "meta:\n  id: a\n  imports:\n    - b\n",
      "/sub/b.ksy": "meta:\n  id: b\n",
    });
    const graph = await collectImportGraph("/main.ksy", vfs);
    expect(graph.map((f) => f.importName).sort()).toEqual([
      "main",
      "sub/a",
      "sub/b",
    ]);
  });

  it("handles import cycles", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/a.ksy": "meta:\n  id: a\n  imports:\n    - b\n",
      "/b.ksy": "meta:\n  id: b\n  imports:\n    - a\n",
    });
    const graph = await collectImportGraph("/a.ksy", vfs);
    expect(graph.map((f) => f.importName).sort()).toEqual(["a", "b"]);
  });

  it("excludes files outside the import graph", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/main.ksy": "meta:\n  id: main\n",
      "/unrelated.ksy": "meta:\n  id: unrelated\n",
    });
    const graph = await collectImportGraph("/main.ksy", vfs);
    expect(graph.map((f) => f.importName)).toEqual(["main"]);
  });

  it("skips imports that don't resolve", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/main.ksy": "meta:\n  id: main\n  imports:\n    - missing\n    - a\n",
      "/a.ksy": "meta:\n  id: a\n",
    });
    const graph = await collectImportGraph("/main.ksy", vfs);
    expect(graph.map((f) => f.importName).sort()).toEqual(["a", "main"]);
  });

  it("follows absolute imports against the VFS root (kaitai-formats style)", async () => {
    const vfs = InMemoryVFS.fromSeed({
      "/filesystem/vfat.ksy":
        "meta:\n  id: vfat\n  imports:\n    - /common/dos_datetime\n",
      "/common/dos_datetime.ksy": "meta:\n  id: dos_datetime\n",
    });
    const graph = await collectImportGraph("/filesystem/vfat.ksy", vfs);
    expect(graph.map((f) => f.importName).sort()).toEqual([
      "common/dos_datetime",
      "filesystem/vfat",
    ]);
  });
});
