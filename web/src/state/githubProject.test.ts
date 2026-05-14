import { describe, expect, it } from "vitest";

import { parseGitHubSpec } from "./githubProject";

describe("parseGitHubSpec", () => {
  it("accepts the short owner/repo form", () => {
    expect(parseGitHubSpec("kaitai-io/kaitai_struct_formats")).toEqual({
      owner: "kaitai-io",
      repo: "kaitai_struct_formats",
      ref: undefined,
    });
  });

  it("accepts owner/repo@ref", () => {
    expect(parseGitHubSpec("kaitai-io/kaitai_struct_formats@main")).toEqual({
      owner: "kaitai-io",
      repo: "kaitai_struct_formats",
      ref: "main",
    });
  });

  it("accepts a github.com URL", () => {
    expect(parseGitHubSpec("https://github.com/foo/bar")).toEqual({
      owner: "foo",
      repo: "bar",
      ref: undefined,
    });
  });

  it("accepts a github.com /tree/ URL with a multi-segment branch name", () => {
    expect(
      parseGitHubSpec("https://github.com/foo/bar/tree/feature/branch-name"),
    ).toEqual({
      owner: "foo",
      repo: "bar",
      ref: "feature/branch-name",
    });
  });

  it("strips a .git suffix from clone-style URLs", () => {
    expect(parseGitHubSpec("https://github.com/foo/bar.git")).toEqual({
      owner: "foo",
      repo: "bar",
      ref: undefined,
    });
  });

  it("rejects non-github.com URLs", () => {
    expect(() => parseGitHubSpec("https://gitlab.com/foo/bar")).toThrow(
      /github\.com/,
    );
  });

  it("rejects nonsense", () => {
    expect(() => parseGitHubSpec("")).toThrow();
    expect(() => parseGitHubSpec("not a spec")).toThrow();
    expect(() => parseGitHubSpec("owner-only")).toThrow();
  });
});
