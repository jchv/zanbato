import { describe, expect, it, vi } from "vitest";

import { InMemoryVFS } from "./memory";
import { OverlayVFS } from "./overlay";

function paths(entries: Array<{ path: string }>): string[] {
  return entries.map((e) => e.path);
}

describe("OverlayVFS - read pass-through", () => {
  it("reads from base when no overlay", async () => {
    const base = InMemoryVFS.fromSeed({ "/a.ksy": "from base" });
    const overlay = new OverlayVFS(base);
    expect(await overlay.readText("/a.ksy")).toBe("from base");
  });

  it("returns base listing when overlay is empty", async () => {
    const base = InMemoryVFS.fromSeed({
      "/a.ksy": "a",
      "/dir/b.ksy": "b",
    });
    const overlay = new OverlayVFS(base);
    expect(paths(await overlay.list("/"))).toEqual(["/dir", "/a.ksy"]);
  });

  it("exists pass-through", async () => {
    const base = InMemoryVFS.fromSeed({ "/a.ksy": "a" });
    const overlay = new OverlayVFS(base);
    expect(await overlay.exists("/a.ksy")).toBe(true);
    expect(await overlay.exists("/missing")).toBe(false);
  });
});

describe("OverlayVFS - writes go to overlay", () => {
  it("write doesn't mutate the base", async () => {
    const base = InMemoryVFS.fromSeed({ "/a.ksy": "from base" });
    const overlay = new OverlayVFS(base);
    await overlay.write("/a.ksy", "overlaid");
    expect(await overlay.readText("/a.ksy")).toBe("overlaid");
    expect(await base.readText("/a.ksy")).toBe("from base");
  });

  it("writes appear in list", async () => {
    const base = InMemoryVFS.fromSeed({ "/a.ksy": "a" });
    const overlay = new OverlayVFS(base);
    await overlay.write("/b.ksy", "b");
    expect(paths(await overlay.list("/"))).toEqual(["/a.ksy", "/b.ksy"]);
  });

  it("overlay creating a deeply-nested file surfaces ancestor dirs", async () => {
    const overlay = new OverlayVFS(new InMemoryVFS());
    await overlay.write("/x/y/z.ksy", "z");
    expect(paths(await overlay.list("/"))).toEqual(["/x"]);
    expect(paths(await overlay.list("/x"))).toEqual(["/x/y"]);
    expect(paths(await overlay.list("/x/y"))).toEqual(["/x/y/z.ksy"]);
  });
});

describe("OverlayVFS - deletes use tombstones", () => {
  it("deletes a base file by tombstoning it", async () => {
    const base = InMemoryVFS.fromSeed({ "/a.ksy": "a", "/b.ksy": "b" });
    const overlay = new OverlayVFS(base);
    await overlay.delete("/a.ksy");
    expect(await overlay.exists("/a.ksy")).toBe(false);
    expect(await overlay.exists("/b.ksy")).toBe(true);
    expect(paths(await overlay.list("/"))).toEqual(["/b.ksy"]);
    // Base is untouched.
    expect(await base.exists("/a.ksy")).toBe(true);
  });

  it("deleting a directory hides all base descendants", async () => {
    const base = InMemoryVFS.fromSeed({
      "/dir/a.ksy": "a",
      "/dir/sub/b.ksy": "b",
      "/other.ksy": "o",
    });
    const overlay = new OverlayVFS(base);
    await overlay.delete("/dir");
    expect(await overlay.exists("/dir")).toBe(false);
    expect(await overlay.exists("/dir/a.ksy")).toBe(false);
    expect(await overlay.exists("/dir/sub/b.ksy")).toBe(false);
    expect(await overlay.exists("/other.ksy")).toBe(true);
  });

  it("writing back to a tombstoned path resurrects it", async () => {
    const base = InMemoryVFS.fromSeed({ "/a.ksy": "from base" });
    const overlay = new OverlayVFS(base);
    await overlay.delete("/a.ksy");
    expect(await overlay.exists("/a.ksy")).toBe(false);
    await overlay.write("/a.ksy", "fresh");
    expect(await overlay.readText("/a.ksy")).toBe("fresh");
  });

  it("ancestor tombstone hides base descendants but overlay writes survive", async () => {
    const base = InMemoryVFS.fromSeed({
      "/dir/a.ksy": "a-base",
      "/dir/b.ksy": "b-base",
    });
    const overlay = new OverlayVFS(base);
    await overlay.delete("/dir");
    expect(await overlay.exists("/dir/a.ksy")).toBe(false);
    // User adds a fresh file under the deleted directory.
    await overlay.write("/dir/c.ksy", "c-new");
    expect(await overlay.readText("/dir/c.ksy")).toBe("c-new");
    // /dir/a.ksy and /dir/b.ksy still tombstoned via the ancestor.
    expect(await overlay.exists("/dir/a.ksy")).toBe(false);
    expect(await overlay.exists("/dir/b.ksy")).toBe(false);
    // /dir reappears because overlay has descendants there.
    expect(await overlay.exists("/dir")).toBe(true);
    // Only the overlay entry shows in the listing.
    expect(paths(await overlay.list("/dir"))).toEqual(["/dir/c.ksy"]);
  });
});

describe("OverlayVFS - subscribe", () => {
  it("fires on overlay writes", async () => {
    const overlay = new OverlayVFS(new InMemoryVFS());
    const cb = vi.fn();
    overlay.subscribe(cb);
    await overlay.write("/a.ksy", "x");
    expect(cb).toHaveBeenCalledWith({ path: "/a.ksy", kind: "write" });
  });

  it("fires on deletes of base-only paths (tombstone-only)", async () => {
    const base = InMemoryVFS.fromSeed({ "/a.ksy": "a" });
    const overlay = new OverlayVFS(base);
    const cb = vi.fn();
    overlay.subscribe(cb);
    await overlay.delete("/a.ksy");
    expect(cb).toHaveBeenCalledWith({ path: "/a.ksy", kind: "delete" });
  });
});

describe("OverlayVFS - introspection accessors", () => {
  it("exposes base, overlay, and tombstone list for commit-construction", async () => {
    const base = InMemoryVFS.fromSeed({ "/a.ksy": "a", "/b.ksy": "b" });
    const overlay = new OverlayVFS(base);
    await overlay.write("/a.ksy", "edited");
    await overlay.delete("/b.ksy");
    await overlay.write("/c.ksy", "new");
    expect(overlay.baseVfs).toBe(base);
    expect(await overlay.overlayVfs.readText("/a.ksy")).toBe("edited");
    expect(await overlay.overlayVfs.readText("/c.ksy")).toBe("new");
    expect(overlay.tombstoneList).toEqual(["/b.ksy"]);
  });
});
