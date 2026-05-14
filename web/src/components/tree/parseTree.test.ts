import { describe, expect, it } from "vitest";

import {
  type TreeNode,
  ancestorsOf,
  findDeepestNodeAtOffset,
  findNodeByPath,
  formatValue,
  parseTreeJson,
} from "./parseTree";

function sampleTree(): TreeNode {
  return {
    name: "smoke",
    path: "smoke",
    kind: "struct",
    range: { startIndex: 0, endIndex: 8 },
    children: [
      {
        name: "magic",
        path: "smoke.magic",
        kind: "bytes",
        value: [0xca, 0xfe],
        range: { startIndex: 0, endIndex: 2 },
      },
      {
        name: "count",
        path: "smoke.count",
        kind: "uint",
        value: 4,
        range: { startIndex: 2, endIndex: 4 },
      },
      {
        name: "items",
        path: "smoke.items",
        kind: "array",
        range: { startIndex: 4, endIndex: 8 },
        children: [
          {
            name: "items[0]",
            path: "smoke.items[0]",
            kind: "uint",
            value: 1,
            range: { startIndex: 4, endIndex: 5 },
          },
          {
            name: "items[1]",
            path: "smoke.items[1]",
            kind: "uint",
            value: 2,
            range: { startIndex: 5, endIndex: 6 },
          },
          {
            name: "items[2]",
            path: "smoke.items[2]",
            kind: "uint",
            value: 3,
            range: { startIndex: 6, endIndex: 7 },
          },
          {
            name: "items[3]",
            path: "smoke.items[3]",
            kind: "uint",
            value: 4,
            range: { startIndex: 7, endIndex: 8 },
          },
        ],
      },
    ],
  };
}

describe("parseTreeJson", () => {
  it("returns the parsed root", () => {
    const json = JSON.stringify({ name: "x", path: "x" });
    expect(parseTreeJson(json)?.name).toBe("x");
  });

  it("returns null on malformed JSON", () => {
    expect(parseTreeJson("not json")).toBeNull();
  });
});

describe("findNodeByPath", () => {
  it("finds the root", () => {
    const t = sampleTree();
    expect(findNodeByPath(t, "smoke")?.name).toBe("smoke");
  });

  it("finds a leaf", () => {
    const t = sampleTree();
    expect(findNodeByPath(t, "smoke.items[2]")?.value).toBe(3);
  });

  it("returns null for unknown paths", () => {
    const t = sampleTree();
    expect(findNodeByPath(t, "smoke.nope")).toBeNull();
  });
});

describe("findDeepestNodeAtOffset", () => {
  it("picks the deepest match", () => {
    const t = sampleTree();
    // Offset 5 is inside items[1], which is inside items, which is inside smoke.
    expect(findDeepestNodeAtOffset(t, 5)?.path).toBe("smoke.items[1]");
  });

  it("falls back to the containing array when an item's range is missing", () => {
    const t = sampleTree();
    // Strip the range from items[2]; offset 6 should now resolve to its
    // parent "items" since items[2] no longer reports a known range.
    (t.children![2]!.children![2]!.range as unknown) = undefined;
    expect(findDeepestNodeAtOffset(t, 6)?.path).toBe("smoke.items");
  });

  it("returns null for offsets outside the buffer", () => {
    const t = sampleTree();
    expect(findDeepestNodeAtOffset(t, 99)).toBeNull();
  });

  it("returns the immediate parent when offset hits a known struct but no specific child", () => {
    // Construct a tree where the root has a range that's wider than its
    // children's combined ranges (i.e., trailing unparsed bytes).
    const t: TreeNode = {
      name: "r",
      path: "r",
      kind: "struct",
      range: { startIndex: 0, endIndex: 10 },
      children: [
        {
          name: "a",
          path: "r.a",
          kind: "uint",
          value: 1,
          range: { startIndex: 0, endIndex: 2 },
        },
      ],
    };
    // Offset 5 isn't in any child but is in the root's range.
    expect(findDeepestNodeAtOffset(t, 5)?.path).toBe("r");
  });

  it("descends past rangeless nodes", () => {
    // A struct with no range itself but child ranges should still be
    // searchable.
    const t: TreeNode = {
      name: "r",
      path: "r",
      kind: "struct",
      children: [
        {
          name: "a",
          path: "r.a",
          kind: "uint",
          value: 1,
          range: { startIndex: 0, endIndex: 2 },
        },
      ],
    };
    expect(findDeepestNodeAtOffset(t, 1)?.path).toBe("r.a");
  });
});

describe("ancestorsOf", () => {
  it("returns the chain from root to immediate parent, excluding the target", () => {
    const t = sampleTree();
    expect(ancestorsOf(t, "smoke.items[2]")).toEqual(["smoke", "smoke.items"]);
  });

  it("returns empty for the root", () => {
    const t = sampleTree();
    expect(ancestorsOf(t, "smoke")).toEqual([]);
  });

  it("returns empty for unknown paths", () => {
    const t = sampleTree();
    expect(ancestorsOf(t, "nope")).toEqual([]);
  });
});

describe("formatValue", () => {
  it("renders integers with both bases when large", () => {
    const node = {
      name: "x",
      path: "x",
      kind: "uint",
      value: 0x1234,
    };
    expect(formatValue(node)).toBe("4660 (0x1234)");
  });

  it("renders small integers without the hex annotation", () => {
    const node = { name: "x", path: "x", kind: "uint", value: 5 };
    expect(formatValue(node)).toBe("5");
  });

  it("renders booleans as words", () => {
    expect(
      formatValue({ name: "x", path: "x", kind: "bool", value: true }),
    ).toBe("true");
  });

  it("renders bytes with a length-bounded preview", () => {
    expect(
      formatValue({
        name: "x",
        path: "x",
        kind: "bytes",
        value: [0xca, 0xfe],
      }),
    ).toBe("[CA FE]");
  });

  it("renders arrays as a count summary", () => {
    expect(
      formatValue({
        name: "x",
        path: "x",
        kind: "array",
        children: [
          { name: "0", path: "x[0]" },
          { name: "1", path: "x[1]" },
        ],
      }),
    ).toBe("[2 items]");
  });

  it("renders error labels", () => {
    expect(formatValue({ name: "x", path: "x", error: "out of range" })).toBe(
      "error: out of range",
    );
  });

  it("renders structs as empty (children speak for themselves)", () => {
    expect(formatValue({ name: "x", path: "x", kind: "struct" })).toBe("");
  });

  it("renders enums with type, label, and integer value", () => {
    expect(
      formatValue({
        name: "marker",
        path: "marker",
        kind: "enum",
        value: { int: 216, label: "soi", enum: "marker" },
      }),
    ).toBe("marker::soi (216)");
  });

  it("renders unknown enum values with the type name as context", () => {
    expect(
      formatValue({
        name: "marker",
        path: "marker",
        kind: "enum",
        value: { int: 17, label: "", enum: "marker" },
      }),
    ).toBe("17 (no match in marker)");
  });
});
