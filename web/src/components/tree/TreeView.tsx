import { useEffect, useMemo, useRef, useState } from "react";

import { type TreeNode, ancestorsOf, formatValue } from "./parseTree";
import "./tree.css";

export interface TreeViewProps {
  root: TreeNode;
  selectedPath: string | null;
  onSelect: (path: string) => void;
}

export function TreeView({ root, selectedPath, onSelect }: TreeViewProps) {
  const [userExpanded, setUserExpanded] = useState<Set<string>>(
    () => new Set([root.path]),
  );

  const expanded = useMemo<Set<string>>(() => {
    const set = new Set(userExpanded);
    set.add(root.path);
    if (selectedPath) {
      for (const p of ancestorsOf(root, selectedPath)) set.add(p);
    }
    return set;
  }, [userExpanded, root, selectedPath]);

  const onToggle = (path: string) => {
    setUserExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  const selectedRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!selectedPath) return;
    selectedRef.current?.scrollIntoView({ block: "nearest" });
  }, [selectedPath]);

  return (
    <div className="tree-view" role="tree">
      <TreeRows
        node={root}
        depth={0}
        expanded={expanded}
        selectedPath={selectedPath}
        onSelect={onSelect}
        onToggle={onToggle}
        selectedRef={selectedRef}
      />
    </div>
  );
}

interface TreeRowsProps {
  node: TreeNode;
  depth: number;
  expanded: Set<string>;
  selectedPath: string | null;
  onSelect: (path: string) => void;
  onToggle: (path: string) => void;
  selectedRef: React.RefObject<HTMLDivElement | null>;
}

function TreeRows({
  node,
  depth,
  expanded,
  selectedPath,
  onSelect,
  onToggle,
  selectedRef,
}: TreeRowsProps) {
  const hasChildren = !!(node.children && node.children.length > 0);
  const isExpanded = expanded.has(node.path);
  const isSelected = selectedPath === node.path;
  const value = formatValue(node);

  return (
    <>
      <div
        ref={isSelected ? selectedRef : undefined}
        className={"tree-row" + (isSelected ? " tree-row--selected" : "")}
        role="treeitem"
        aria-expanded={hasChildren ? isExpanded : undefined}
        aria-selected={isSelected}
        style={{ paddingLeft: `${depth * 14 + 4}px` }}
        onClick={() => onSelect(node.path)}
        onDoubleClick={() => hasChildren && onToggle(node.path)}
      >
        {hasChildren ? (
          <button
            type="button"
            className="tree-disclosure"
            aria-label={isExpanded ? "Collapse" : "Expand"}
            onClick={(e) => {
              e.stopPropagation();
              onToggle(node.path);
            }}
          >
            {isExpanded ? "▾" : "▸"}
          </button>
        ) : (
          <span className="tree-disclosure tree-disclosure--leaf" />
        )}
        <span className="tree-name">{node.name}</span>
        {node.typeName && (
          <span className="tree-type-name">: {node.typeName}</span>
        )}
        {node.kind && !node.typeName && (
          <span className="tree-kind">{node.kind}</span>
        )}
        {value && <span className="tree-value">{value}</span>}
      </div>
      {hasChildren && isExpanded && (
        <>
          {node.children!.map((child) => (
            <TreeRows
              key={child.path}
              node={child}
              depth={depth + 1}
              expanded={expanded}
              selectedPath={selectedPath}
              onSelect={onSelect}
              onToggle={onToggle}
              selectedRef={selectedRef}
            />
          ))}
        </>
      )}
    </>
  );
}
