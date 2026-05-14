import { describe, expect, it, vi } from "vitest";

import { InMemoryVFS } from "./memory";
import { basename, dirname, normalizePath } from "./types";

function paths(entries: Array<{ path: string }>): string[] {
  return entries.map((e) => e.path);
}

describe("normalizePath", () => {
  it("prepends a leading slash", () => {
    expect(normalizePath("foo")).toBe("/foo");
    expect(normalizePath("foo/bar")).toBe("/foo/bar");
  });

  it("strips trailing slashes", () => {
    expect(normalizePath("/foo/")).toBe("/foo");
    expect(normalizePath("/foo///")).toBe("/foo");
  });

  it("preserves the root", () => {
    expect(normalizePath("/")).toBe("/");
    expect(normalizePath("")).toBe("/");
  });
});

describe("basename + dirname", () => {
  it("basename returns the last segment", () => {
    expect(basename("/a/b/c.ksy")).toBe("c.ksy");
    expect(basename("/x")).toBe("x");
    expect(basename("/")).toBe("");
  });

  it("dirname returns the parent", () => {
    expect(dirname("/a/b/c.ksy")).toBe("/a/b");
    expect(dirname("/x")).toBe("/");
    expect(dirname("/")).toBe("/");
  });
});

describe("InMemoryVFS", () => {
  it("starts empty", async () => {
    const vfs = new InMemoryVFS();
    expect(await vfs.list("/")).toEqual([]);
    expect(await vfs.exists("/anything")).toBe(false);
    expect(await vfs.exists("/")).toBe(true);
  });

  it("writes and reads text", async () => {
    const vfs = new InMemoryVFS();
    await vfs.write("/foo.ksy", "meta: {id: foo}");
    expect(await vfs.readText("/foo.ksy")).toBe("meta: {id: foo}");
  });

  it("writes and reads bytes", async () => {
    const vfs = new InMemoryVFS();
    await vfs.write("/foo.bin", new Uint8Array([1, 2, 3]));
    expect(Array.from(await vfs.read("/foo.bin"))).toEqual([1, 2, 3]);
  });

  it("copies input bytes - caller mutation doesn't affect the stored copy", async () => {
    const vfs = new InMemoryVFS();
    const src = new Uint8Array([1, 2, 3]);
    await vfs.write("/foo.bin", src);
    src[0] = 99;
    expect(Array.from(await vfs.read("/foo.bin"))).toEqual([1, 2, 3]);
  });

  it("rejects reads of missing files", async () => {
    const vfs = new InMemoryVFS();
    await expect(vfs.read("/nope")).rejects.toThrow(/File not found/);
  });

  it("normalizes paths on both sides of write and read", async () => {
    const vfs = new InMemoryVFS();
    await vfs.write("foo.ksy", "x");
    expect(await vfs.readText("/foo.ksy")).toBe("x");
    expect(await vfs.readText("foo.ksy")).toBe("x");
  });

  it("rejects writing to /", async () => {
    const vfs = new InMemoryVFS();
    await expect(vfs.write("/", "x")).rejects.toThrow();
  });

  describe("list", () => {
    it("returns immediate children only", async () => {
      const vfs = InMemoryVFS.fromSeed({
        "/a.ksy": "a",
        "/dir/b.ksy": "b",
        "/dir/sub/c.ksy": "c",
      });
      expect(paths(await vfs.list("/"))).toEqual(["/dir", "/a.ksy"]);
      expect(paths(await vfs.list("/dir"))).toEqual(["/dir/sub", "/dir/b.ksy"]);
      expect(paths(await vfs.list("/dir/sub"))).toEqual(["/dir/sub/c.ksy"]);
    });

    it("returns directories first, then files, alphabetically within each group", async () => {
      const vfs = InMemoryVFS.fromSeed({
        "/z.ksy": "z",
        "/m.ksy": "m",
        "/b/inner.ksy": "i",
        "/a/inner.ksy": "i",
      });
      expect(paths(await vfs.list("/"))).toEqual([
        "/a",
        "/b",
        "/m.ksy",
        "/z.ksy",
      ]);
    });

    it("returns each entry with its display name", async () => {
      const vfs = InMemoryVFS.fromSeed({
        "/dir/file.ksy": "x",
      });
      const root = await vfs.list("/");
      expect(root).toEqual([{ path: "/dir", name: "dir", type: "directory" }]);
      const inner = await vfs.list("/dir");
      expect(inner).toEqual([
        { path: "/dir/file.ksy", name: "file.ksy", type: "file" },
      ]);
    });

    it("returns an empty list for an unknown directory", async () => {
      const vfs = new InMemoryVFS();
      expect(await vfs.list("/nope")).toEqual([]);
    });
  });

  describe("exists", () => {
    it("returns true for files we have", async () => {
      const vfs = InMemoryVFS.fromSeed({ "/a/b.ksy": "x" });
      expect(await vfs.exists("/a/b.ksy")).toBe(true);
    });

    it("returns true for directories that contain files", async () => {
      const vfs = InMemoryVFS.fromSeed({ "/a/b.ksy": "x" });
      expect(await vfs.exists("/a")).toBe(true);
    });

    it("returns false for unrelated paths", async () => {
      const vfs = InMemoryVFS.fromSeed({ "/a/b.ksy": "x" });
      expect(await vfs.exists("/a/c.ksy")).toBe(false);
      expect(await vfs.exists("/c")).toBe(false);
    });
  });

  describe("delete", () => {
    it("removes a file", async () => {
      const vfs = InMemoryVFS.fromSeed({ "/a.ksy": "x", "/b.ksy": "y" });
      await vfs.delete("/a.ksy");
      expect(await vfs.exists("/a.ksy")).toBe(false);
      expect(await vfs.exists("/b.ksy")).toBe(true);
    });

    it("removes everything beneath a directory", async () => {
      const vfs = InMemoryVFS.fromSeed({
        "/dir/a.ksy": "a",
        "/dir/sub/b.ksy": "b",
        "/other.ksy": "o",
      });
      await vfs.delete("/dir");
      expect(await vfs.exists("/dir")).toBe(false);
      expect(await vfs.exists("/dir/a.ksy")).toBe(false);
      expect(await vfs.exists("/dir/sub/b.ksy")).toBe(false);
      expect(await vfs.exists("/other.ksy")).toBe(true);
    });

    it("delete(/) wipes everything", async () => {
      const vfs = InMemoryVFS.fromSeed({ "/a.ksy": "x", "/b.ksy": "y" });
      await vfs.delete("/");
      expect(paths(await vfs.list("/"))).toEqual([]);
    });

    it("missing paths are silent", async () => {
      const vfs = new InMemoryVFS();
      await expect(vfs.delete("/nope")).resolves.toBeUndefined();
    });
  });

  describe("mkdir", () => {
    it("creates an empty directory visible to list()", async () => {
      const vfs = new InMemoryVFS();
      await vfs.mkdir("/empty");
      expect(paths(await vfs.list("/"))).toEqual(["/empty"]);
      expect(await vfs.exists("/empty")).toBe(true);
      // The directory itself has no children.
      expect(await vfs.list("/empty")).toEqual([]);
    });

    it("is idempotent if the directory already exists implicitly", async () => {
      const vfs = InMemoryVFS.fromSeed({ "/dir/a.ksy": "x" });
      await vfs.mkdir("/dir");
      expect(paths(await vfs.list("/"))).toEqual(["/dir"]);
      expect(paths(await vfs.list("/dir"))).toEqual(["/dir/a.ksy"]);
    });

    it("writing a file inside an empty directory drops the marker", async () => {
      const vfs = new InMemoryVFS();
      await vfs.mkdir("/x/y");
      await vfs.write("/x/y/z.ksy", "z");
      // Deleting only the file should make /x/y also disappear.
      await vfs.delete("/x/y/z.ksy");
      expect(await vfs.exists("/x/y")).toBe(false);
    });

    it("delete on an empty directory removes its marker", async () => {
      const vfs = new InMemoryVFS();
      await vfs.mkdir("/empty");
      await vfs.delete("/empty");
      expect(await vfs.exists("/empty")).toBe(false);
    });
  });

  describe("subscribe", () => {
    it("fires on write", async () => {
      const vfs = new InMemoryVFS();
      const cb = vi.fn();
      vfs.subscribe(cb);
      await vfs.write("/a.ksy", "x");
      expect(cb).toHaveBeenCalledWith({ path: "/a.ksy", kind: "write" });
    });

    it("fires on delete", async () => {
      const vfs = InMemoryVFS.fromSeed({ "/a.ksy": "x" });
      const cb = vi.fn();
      vfs.subscribe(cb);
      await vfs.delete("/a.ksy");
      expect(cb).toHaveBeenCalledWith({ path: "/a.ksy", kind: "delete" });
    });

    it("doesn't fire on delete-of-missing", async () => {
      const vfs = new InMemoryVFS();
      const cb = vi.fn();
      vfs.subscribe(cb);
      await vfs.delete("/nope");
      expect(cb).not.toHaveBeenCalled();
    });

    it("unsubscribe stops further notifications", async () => {
      const vfs = new InMemoryVFS();
      const cb = vi.fn();
      const off = vfs.subscribe(cb);
      await vfs.write("/a.ksy", "x");
      off();
      await vfs.write("/b.ksy", "y");
      expect(cb).toHaveBeenCalledTimes(1);
    });
  });
});
