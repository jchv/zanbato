export interface TreeRange {
  startIndex: number;
  endIndex: number;
}

export interface TreeNode {
  name: string;
  path: string;
  kind?: string;
  typeName?: string;
  value?: unknown;
  range?: TreeRange;
  error?: string;
  children?: TreeNode[];
}

export function parseTreeJson(json: string): TreeNode | null {
  try {
    return JSON.parse(json) as TreeNode;
  } catch {
    return null;
  }
}

/**
 * Depth-first search for a node by its `path`. Returns null if no match.
 */
export function findNodeByPath(root: TreeNode, path: string): TreeNode | null {
  if (root.path === path) return root;
  if (!root.children) return null;
  for (const child of root.children) {
    const hit = findNodeByPath(child, path);
    if (hit) return hit;
  }
  return null;
}

export function findDeepestNodeAtOffset(
  root: TreeNode,
  offset: number,
): TreeNode | null {
  // If this node has a range and offset is outside, skip the subtree.
  if (
    root.range &&
    (offset < root.range.startIndex || offset >= root.range.endIndex)
  ) {
    return null;
  }
  if (root.children) {
    for (const child of root.children) {
      const hit = findDeepestNodeAtOffset(child, offset);
      if (hit) return hit;
    }
  }
  return root.range ? root : null;
}

export function ancestorsOf(root: TreeNode, targetPath: string): string[] {
  const out: string[] = [];
  function walk(node: TreeNode, chain: string[]): boolean {
    if (node.path === targetPath) {
      // The chain doesn't include the target itself - only its ancestors.
      out.push(...chain);
      return true;
    }
    if (!node.children) return false;
    chain.push(node.path);
    for (const child of node.children) {
      if (walk(child, chain)) return true;
    }
    chain.pop();
    return false;
  }
  walk(root, []);
  return out;
}

export function formatValue(node: TreeNode): string {
  if (node.error) return `error: ${node.error}`;
  if (node.kind === "array") {
    const count = node.children?.length ?? 0;
    return `[${count} item${count === 1 ? "" : "s"}]`;
  }
  if (node.kind === "struct") return "";
  if (node.value === undefined) return "";
  switch (node.kind) {
    case "int":
    case "uint": {
      const n = node.value as number;
      if (typeof n === "number" && Math.abs(n) >= 10) {
        return `${n} (0x${Math.abs(n).toString(16).toUpperCase()})`;
      }
      return String(n);
    }
    case "float":
      return String(node.value);
    case "bool":
      return node.value ? "true" : "false";
    case "str":
      return JSON.stringify(node.value);
    case "bytes": {
      const v = node.value;
      let arr: number[];
      if (v == null) return "[]";
      if (Array.isArray(v)) {
        arr = v as number[];
      } else {
        return `<unexpected bytes value: ${typeof v}>`;
      }
      const preview = arr
        .slice(0, 12)
        .map((b) => b.toString(16).padStart(2, "0").toUpperCase())
        .join(" ");
      return arr.length > 12
        ? `[${preview} ... (${arr.length} bytes)]`
        : `[${preview}]`;
    }
    case "enum": {
      const v = node.value as { int: number; label: string; enum: string };
      if (v.label) {
        return `${v.enum}::${v.label} (${v.int})`;
      }
      return `${v.int} (no match in ${v.enum})`;
    }
    default:
      return String(node.value);
  }
}
