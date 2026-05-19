import { execSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";
import * as vm from "node:vm";
import { beforeAll, describe, expect, it } from "vitest";

interface ZanbatoAPI {
  loadKsys: (
    files: Array<{ name: string; source: string }>,
  ) => { ok: boolean; error?: string };
  parse: (
    rootName: string,
    data: Uint8Array,
  ) => { ok: boolean; tree?: string; error?: string };
}

declare global {
  var Go: new () => {
    importObject: WebAssembly.Imports;
    run(inst: WebAssembly.Instance): Promise<void>;
  };
  var zanbato: ZanbatoAPI | undefined;
}

const wasmPath = path.resolve(__dirname, "../../public/zanbato.wasm");
const haveWasm = fs.existsSync(wasmPath);

(haveWasm ? describe : describe.skip)("wasm runtime", () => {
  beforeAll(async () => {
    const goRoot = execSync("go env GOROOT", { encoding: "utf8" }).trim();
    const wasmExec = path.join(goRoot, "lib/wasm/wasm_exec.js");
    vm.runInThisContext(fs.readFileSync(wasmExec, "utf8"));

    const wasmBuf = fs.readFileSync(wasmPath);
    const go = new globalThis.Go();
    const { instance } = await WebAssembly.instantiate(
      wasmBuf,
      go.importObject,
    );
    void go.run(instance);

    // Wait for the API to appear on globalThis.
    const start = Date.now();
    while (typeof globalThis.zanbato === "undefined") {
      if (Date.now() - start > 5000) {
        throw new Error("wasm did not register zanbato global within 5s");
      }
      await new Promise((r) => setTimeout(r, 5));
    }
  }, 30_000);

  it("round-trips a trivial KSY -> binary -> tree", () => {
    const api = globalThis.zanbato!;

    const ksy = [
      "meta:",
      "  id: smoke",
      "seq:",
      "  - id: x",
      "    type: u1",
      "  - id: y",
      "    type: u2le",
      "",
    ].join("\n");
    const loaded = api.loadKsys([{ name: "smoke", source: ksy }]);
    expect(loaded).toEqual({ ok: true });

    const parsed = api.parse("smoke", new Uint8Array([0x2a, 0x34, 0x12]));
    expect(parsed.ok).toBe(true);
    expect(parsed.error).toBeUndefined();

    const tree = JSON.parse(parsed.tree!);
    expect(tree.kind).toBe("struct");
    expect(tree.children).toHaveLength(2);
    expect(tree.children[0].name).toBe("x");
    expect(tree.children[0].value).toBe(42);
    expect(tree.children[1].name).toBe("y");
    expect(tree.children[1].value).toBe(0x1234);
  });

  it("returns an error result for an unloaded root name", () => {
    const api = globalThis.zanbato!;
    api.loadKsys([]);
    const parsed = api.parse("does_not_exist", new Uint8Array([0]));
    expect(parsed.ok).toBe(false);
    expect(parsed.error).toBeTruthy();
  });

  it("parses the app's seed KSY + binary (regression for the frontend)", () => {
    const api = globalThis.zanbato!;
    const ksy = [
      "meta:",
      "  id: smoke",
      "seq:",
      "  - id: magic",
      "    contents: [0xCA, 0xFE]",
      "  - id: count",
      "    type: u2le",
      "  - id: items",
      "    type: u1",
      "    repeat: expr",
      "    repeat-expr: count",
      "",
    ].join("\n");
    expect(api.loadKsys([{ name: "smoke", source: ksy }])).toEqual({ ok: true });

    const parsed = api.parse(
      "smoke",
      new Uint8Array([0xca, 0xfe, 0x04, 0x00, 1, 2, 3, 4]),
    );
    expect(parsed.ok).toBe(true);
    expect(parsed.error).toBeUndefined();
    const tree = JSON.parse(parsed.tree!);
    expect(tree.kind).toBe("struct");
    expect(tree.children).toHaveLength(3);
    expect(tree.children[1].name).toBe("count");
    expect(tree.children[1].value).toBe(4);
    expect(tree.children[2].name).toBe("items");
    expect(tree.children[2].children).toHaveLength(4);
    expect(tree.children[1].range).toEqual({ startIndex: 2, endIndex: 4 });
    expect(tree.children[2].range).toEqual({ startIndex: 4, endIndex: 8 });
  });

  it("replaces the VFS atomically on each loadKsys call", () => {
    const api = globalThis.zanbato!;
    api.loadKsys([
      { name: "v", source: "meta: {id: v}\nseq:\n  - {id: a, type: u1}\n" },
    ]);
    const p1 = api.parse("v", new Uint8Array([1]));
    expect(JSON.parse(p1.tree!).children[0].name).toBe("a");

    api.loadKsys([
      { name: "v", source: "meta: {id: v}\nseq:\n  - {id: b, type: u1}\n" },
    ]);
    const p2 = api.parse("v", new Uint8Array([2]));
    expect(JSON.parse(p2.tree!).children[0].name).toBe("b");
  });

  it("resolves absolute imports against the VFS root", () => {
    const api = globalThis.zanbato!;
    const vfat = [
      "meta:",
      "  id: vfat",
      "  imports:",
      "    - /common/dos_datetime",
      "seq:",
      "  - id: t",
      "    type: dos_datetime",
      "",
    ].join("\n");
    const dosDatetime = [
      "meta:",
      "  id: dos_datetime",
      "seq:",
      "  - id: stamp",
      "    type: u2le",
      "",
    ].join("\n");
    expect(
      api.loadKsys([
        { name: "filesystem/vfat", source: vfat },
        { name: "common/dos_datetime", source: dosDatetime },
      ]),
    ).toEqual({ ok: true });

    const parsed = api.parse("filesystem/vfat", new Uint8Array([0x21, 0x43]));
    expect(parsed.ok).toBe(true);
    expect(parsed.error).toBeUndefined();
    const tree = JSON.parse(parsed.tree!);
    expect(tree.children[0].children[0].name).toBe("stamp");
    expect(tree.children[0].children[0].value).toBe(0x4321);
  });

  it("resolves imports across the supplied set", () => {
    const api = globalThis.zanbato!;
    const main = [
      "meta:",
      "  id: main",
      "  imports:",
      "    - helpers/leaf",
      "seq:",
      "  - id: x",
      "    type: leaf",
      "",
    ].join("\n");
    const leaf = [
      "meta:",
      "  id: leaf",
      "seq:",
      "  - id: n",
      "    type: u1",
      "",
    ].join("\n");
    expect(
      api.loadKsys([
        { name: "main", source: main },
        { name: "helpers/leaf", source: leaf },
      ]),
    ).toEqual({ ok: true });

    const parsed = api.parse("main", new Uint8Array([0x05]));
    expect(parsed.ok).toBe(true);
    expect(parsed.error).toBeUndefined();
    const tree = JSON.parse(parsed.tree!);
    expect(tree.children[0].name).toBe("x");
    expect(tree.children[0].children[0].name).toBe("n");
    expect(tree.children[0].children[0].value).toBe(5);
  });
});
