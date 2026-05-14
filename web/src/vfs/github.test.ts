import { describe, expect, it, vi } from "vitest";

import { GitHubVFS } from "./github";

interface TreeEntry {
  path: string;
  type: "blob" | "tree";
  size?: number;
}

function mockFetch(
  treeEntries: TreeEntry[],
  files: Record<string, string> = {},
  opts: { truncated?: boolean; treeStatus?: number; treeBody?: string } = {},
) {
  return vi.fn<typeof fetch>(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    if (url.includes("api.github.com")) {
      const status = opts.treeStatus ?? 200;
      const body =
        opts.treeBody ??
        JSON.stringify({
          sha: "abc",
          tree: treeEntries,
          truncated: opts.truncated ?? false,
        });
      return new Response(body, { status });
    }
    if (url.includes("raw.githubusercontent.com")) {
      // Path is the trailing portion after .../{ref}/.
      const m = url.match(
        /raw\.githubusercontent\.com\/[^/]+\/[^/]+\/[^/]+\/(.+)/,
      );
      const file = m && files[m[1]!];
      if (file === undefined) {
        return new Response("not found", { status: 404 });
      }
      return new Response(file, { status: 200 });
    }
    return new Response("unhandled", { status: 500 });
  });
}

const STD_OPTS = { owner: "kaitai-io", repo: "kaitai_struct_formats" };

describe("GitHubVFS.open", () => {
  it("fetches the recursive tree on construction", async () => {
    const fetchImpl = mockFetch([{ path: "a.ksy", type: "blob" }]);
    const vfs = await GitHubVFS.open({ ...STD_OPTS, fetchImpl });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    const calledUrl = (fetchImpl.mock.calls[0]![0]! as string).toString();
    expect(calledUrl).toContain(
      "api.github.com/repos/kaitai-io/kaitai_struct_formats/git/trees/HEAD?recursive=1",
    );
    expect(vfs.owner).toBe("kaitai-io");
    expect(vfs.ref).toBe("HEAD");
  });

  it("honors a custom ref", async () => {
    const fetchImpl = mockFetch([]);
    await GitHubVFS.open({ ...STD_OPTS, ref: "main", fetchImpl });
    const calledUrl = (fetchImpl.mock.calls[0]![0]! as string).toString();
    expect(calledUrl).toContain("/git/trees/main?");
  });

  it("sends Authorization when a token is provided", async () => {
    const fetchImpl = mockFetch([]);
    await GitHubVFS.open({ ...STD_OPTS, token: "abc123", fetchImpl });
    const headers = (fetchImpl.mock.calls[0]![1]! as RequestInit)
      .headers as Record<string, string>;
    expect(headers["Authorization"]).toBe("Bearer abc123");
  });

  it("throws on non-200 from the trees API", async () => {
    const fetchImpl = mockFetch([], {}, { treeStatus: 404, treeBody: "nope" });
    await expect(GitHubVFS.open({ ...STD_OPTS, fetchImpl })).rejects.toThrow(
      /GitHub API 404/,
    );
  });

  it("throws if the tree was truncated", async () => {
    const fetchImpl = mockFetch([], {}, { truncated: true });
    await expect(GitHubVFS.open({ ...STD_OPTS, fetchImpl })).rejects.toThrow(
      /truncated/,
    );
  });
});

describe("GitHubVFS.list", () => {
  it("returns immediate children of the root", async () => {
    const fetchImpl = mockFetch([
      { path: "README.md", type: "blob" },
      { path: "common/png.ksy", type: "blob" },
      { path: "common", type: "tree" },
    ]);
    const vfs = await GitHubVFS.open({ ...STD_OPTS, fetchImpl });
    const list = await vfs.list("/");
    // Sorted: directories first, then files.
    expect(list.map((e) => e.path)).toEqual(["/common", "/README.md"]);
    expect(list[0]!.type).toBe("directory");
    expect(list[1]!.type).toBe("file");
  });

  it("returns immediate children of a subdirectory", async () => {
    const fetchImpl = mockFetch([
      { path: "common/png.ksy", type: "blob" },
      { path: "common/jpeg.ksy", type: "blob" },
      { path: "common/subdir/inner.ksy", type: "blob" },
    ]);
    const vfs = await GitHubVFS.open({ ...STD_OPTS, fetchImpl });
    expect((await vfs.list("/common")).map((e) => e.path)).toEqual([
      "/common/subdir",
      "/common/jpeg.ksy",
      "/common/png.ksy",
    ]);
  });
});

describe("GitHubVFS.read", () => {
  it("fetches and caches file bodies", async () => {
    const fetchImpl = mockFetch([{ path: "a.ksy", type: "blob" }], {
      "a.ksy": "meta:\n  id: a\n",
    });
    const vfs = await GitHubVFS.open({ ...STD_OPTS, fetchImpl });
    expect(await vfs.readText("/a.ksy")).toBe("meta:\n  id: a\n");
    // Second read should hit the cache, not fetch again.
    fetchImpl.mockClear();
    expect(await vfs.readText("/a.ksy")).toBe("meta:\n  id: a\n");
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it("throws on unknown paths", async () => {
    const fetchImpl = mockFetch([{ path: "a.ksy", type: "blob" }]);
    const vfs = await GitHubVFS.open({ ...STD_OPTS, fetchImpl });
    await expect(vfs.read("/nope.ksy")).rejects.toThrow(/not found/i);
  });

  it("propagates raw.githubusercontent.com errors", async () => {
    const fetchImpl = mockFetch([{ path: "a.ksy", type: "blob" }], {});
    const vfs = await GitHubVFS.open({ ...STD_OPTS, fetchImpl });
    await expect(vfs.read("/a.ksy")).rejects.toThrow(/404/);
  });
});

describe("GitHubVFS - read-only", () => {
  it("write rejects", async () => {
    const fetchImpl = mockFetch([]);
    const vfs = await GitHubVFS.open({ ...STD_OPTS, fetchImpl });
    await expect(vfs.write("/x", "y")).rejects.toThrow(/read-only/);
  });

  it("delete rejects", async () => {
    const fetchImpl = mockFetch([]);
    const vfs = await GitHubVFS.open({ ...STD_OPTS, fetchImpl });
    await expect(vfs.delete("/x")).rejects.toThrow(/read-only/);
  });

  it("mkdir rejects", async () => {
    const fetchImpl = mockFetch([]);
    const vfs = await GitHubVFS.open({ ...STD_OPTS, fetchImpl });
    await expect(vfs.mkdir("/x")).rejects.toThrow(/read-only/);
  });
});
