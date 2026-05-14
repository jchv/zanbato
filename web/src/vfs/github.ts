/**
 * Read-only GitHub-backed VFS.
 *
 * `GitHubVFS.open({owner, repo, ref})` fetches the repository tree
 * recursively (one API call) and caches it. Subsequent `list()` and
 * `exists()` calls answer from the cache without touching the network.
 * `read()` lazily fetches the file content from
 * `raw.githubusercontent.com` - that endpoint doesn't share the
 * authenticated API's rate-limit budget, so it's friendlier for the
 * bulk reads kaitai-struct-formats triggers when the worker syncs every
 * `.ksy` file. File bodies are cached in-process; the second read of
 * the same path is a no-op.
 *
 * All write operations throw - wrap in `OverlayVFS` if you need to
 * stage edits on top of a GitHub-backed project.
 */
import {
  type VFS,
  type VFSEntry,
  type VFSListener,
  normalizePath,
} from "./types";

export interface GitHubVFSOptions {
  owner: string;
  repo: string;
  ref?: string | undefined;
  token?: string | undefined;
  fetchImpl?: typeof fetch | undefined;
}

interface TreeEntry {
  /** Path WITHOUT a leading slash, as the GitHub API returns it. */
  path: string;
  type: "blob" | "tree" | "commit";
  size?: number;
}

export class GitHubVFS implements VFS {
  // file/dir paths (with leading "/") -> entry from the GitHub trees API
  private entries = new Map<string, TreeEntry>();
  private cache = new Map<string, Uint8Array>();
  private listeners = new Set<VFSListener>();
  private readonly fetchImpl: typeof fetch;
  public readonly owner: string;
  public readonly repo: string;
  public readonly ref: string;

  private constructor(
    opts: GitHubVFSOptions & { ref: string },
    tree: TreeEntry[],
  ) {
    this.owner = opts.owner;
    this.repo = opts.repo;
    this.ref = opts.ref;
    this.fetchImpl = opts.fetchImpl ?? globalThis.fetch.bind(globalThis);

    for (const e of tree) {
      this.entries.set("/" + e.path, e);
    }
  }

  static async open(opts: GitHubVFSOptions): Promise<GitHubVFS> {
    const ref = opts.ref ?? "HEAD";
    const fetchImpl = opts.fetchImpl ?? globalThis.fetch.bind(globalThis);
    const url =
      `https://api.github.com/repos/${encodeURIComponent(opts.owner)}` +
      `/${encodeURIComponent(opts.repo)}/git/trees/${encodeURIComponent(ref)}?recursive=1`;
    const headers: Record<string, string> = {
      Accept: "application/vnd.github+json",
      "X-GitHub-Api-Version": "2022-11-28",
    };
    if (opts.token) headers["Authorization"] = `Bearer ${opts.token}`;

    const resp = await fetchImpl(url, { headers });
    if (!resp.ok) {
      const body = await resp.text().catch(() => "");
      throw new Error(
        `GitHub API ${resp.status} ${resp.statusText}: ${body || url}`,
      );
    }
    const data = (await resp.json()) as {
      sha?: string;
      tree: TreeEntry[];
      truncated?: boolean;
    };
    if (data.truncated) {
      // The trees API caps at ~100k entries; very large monorepos hit
      // this. Falling back to per-directory navigation is an option but
      // adds complexity; for now we just surface a clear error.
      throw new Error(
        `GitHub returned a truncated tree for ${opts.owner}/${opts.repo} ` +
          `(>100k entries). Try a smaller subtree or use the contents API path.`,
      );
    }
    return new GitHubVFS({ ...opts, ref }, data.tree);
  }

  async list(path: string): Promise<VFSEntry[]> {
    const dir = normalizePath(path);
    const prefix = dir === "/" ? "/" : dir + "/";
    const seen = new Map<string, VFSEntry>();
    for (const [entryPath, entry] of this.entries) {
      if (!entryPath.startsWith(prefix)) continue;
      const rest = entryPath.slice(prefix.length);
      const slash = rest.indexOf("/");
      if (slash === -1) {
        // Immediate child (file or empty subtree leaf).
        const isDir = entry.type === "tree";
        seen.set(entryPath, {
          path: entryPath,
          name: rest,
          type: isDir ? "directory" : "file",
        });
      } else {
        // Deeper descendant - collapse to its immediate ancestor.
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
    return Array.from(seen.values()).sort(compareEntries);
  }

  async read(path: string): Promise<Uint8Array> {
    const norm = normalizePath(path);
    const cached = this.cache.get(norm);
    if (cached) return cached;
    const entry = this.entries.get(norm);
    if (!entry || entry.type !== "blob") {
      throw new Error(`File not found: ${norm}`);
    }
    // raw.githubusercontent.com is CDN-served, supports CORS, and isn't
    // counted against the authenticated API's rate-limit window. The
    // ref must be URL-safe (which encodeURIComponent handles).
    const repoPath = norm.slice(1); // strip leading "/"
    const url =
      `https://raw.githubusercontent.com/${encodeURIComponent(this.owner)}` +
      `/${encodeURIComponent(this.repo)}/${encodeURIComponent(this.ref)}/${repoPath}`;
    const resp = await this.fetchImpl(url);
    if (!resp.ok) {
      throw new Error(
        `Failed to fetch ${url}: ${resp.status} ${resp.statusText}`,
      );
    }
    const bytes = new Uint8Array(await resp.arrayBuffer());
    this.cache.set(norm, bytes);
    return bytes;
  }

  async readText(path: string): Promise<string> {
    return new TextDecoder().decode(await this.read(path));
  }

  async write(_path: string, _data: Uint8Array | string): Promise<void> {
    throw new Error("GitHubVFS is read-only");
  }
  async mkdir(_path: string): Promise<void> {
    throw new Error("GitHubVFS is read-only");
  }
  async delete(_path: string): Promise<void> {
    throw new Error("GitHubVFS is read-only");
  }

  async exists(path: string): Promise<boolean> {
    const norm = normalizePath(path);
    if (norm === "/") return true;
    if (this.entries.has(norm)) return true;
    const prefix = norm + "/";
    for (const entryPath of this.entries.keys()) {
      if (entryPath.startsWith(prefix)) return true;
    }
    return false;
  }

  subscribe(listener: VFSListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }
}

function compareEntries(a: VFSEntry, b: VFSEntry): number {
  if (a.type !== b.type) return a.type === "directory" ? -1 : 1;
  return a.name.localeCompare(b.name);
}
