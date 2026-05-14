/**
 * Build a Project from a GitHub repo reference.
 *
 * Parses the user-supplied spec (a few shapes accepted), constructs a
 * read-only `GitHubVFS`, wraps it in an `OverlayVFS` so local edits
 * can be staged, and returns a fresh Project with no KSY/binary
 * selected. The caller hands this to `workspace.setProject()`.
 *
 * Accepted spec shapes:
 *   "owner/repo"
 *   "owner/repo@ref"
 *   "https://github.com/owner/repo"
 *   "https://github.com/owner/repo/tree/ref"
 */
import { GitHubVFS } from "../vfs/github";
import { OverlayVFS } from "../vfs/overlay";
import type { Project } from "./project";

export interface ParsedGitHubSpec {
  owner: string;
  repo: string;
  ref?: string | undefined;
}

export function parseGitHubSpec(spec: string): ParsedGitHubSpec {
  const trimmed = spec.trim();
  if (!trimmed) throw new Error("empty spec");

  // Full URL form - use the URL constructor so we naturally handle
  // refs containing slashes (e.g. "feature/some-branch") and any
  // pathological characters via standard URL parsing.
  if (/^https?:\/\//i.test(trimmed)) {
    let u: URL;
    try {
      u = new URL(trimmed);
    } catch {
      throw new Error(`couldn't parse "${spec}" as a URL`);
    }
    if (u.hostname.toLowerCase() !== "github.com") {
      throw new Error(`expected github.com URL, got "${u.hostname}"`);
    }
    const parts = u.pathname.split("/").filter(Boolean);
    const [owner, repo, ...rest] = parts;
    if (!owner || !repo) {
      throw new Error(`URL "${spec}" doesn't have an owner/repo path`);
    }
    // Strip trailing .git if present
    const cleanRepo = repo.endsWith(".git")
      ? repo.slice(0, -".git".length)
      : repo;
    let ref: string | undefined;
    if (rest[0] === "tree" && rest.length > 1) {
      ref = rest.slice(1).join("/");
    }
    return { owner, repo: cleanRepo, ref };
  }

  // "owner/repo[@ref]" form. Refs may contain slashes here too.
  const shortMatch = trimmed.match(/^([^/\s@]+)\/([^/\s@]+)(?:@(.+))?$/);
  if (shortMatch && shortMatch[1] && shortMatch[2]) {
    return {
      owner: shortMatch[1],
      repo: shortMatch[2],
      ref: shortMatch[3],
    };
  }

  throw new Error(
    `couldn't parse "${spec}" - expected owner/repo, owner/repo@ref, or a GitHub URL`,
  );
}

export async function openGitHubProject(
  spec: string,
  opts: { token?: string; fetchImpl?: typeof fetch } = {},
): Promise<Project> {
  const parsed = parseGitHubSpec(spec);
  const base = await GitHubVFS.open({
    owner: parsed.owner,
    repo: parsed.repo,
    ref: parsed.ref,
    token: opts.token,
    fetchImpl: opts.fetchImpl,
  });
  const vfs = new OverlayVFS(base);
  return {
    vfs,
    currentKsyPath: null,
    currentBinaryPath: null,
  };
}
