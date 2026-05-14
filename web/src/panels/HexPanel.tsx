import type { IDockviewPanelProps } from "dockview-react";
import { useCallback, useMemo } from "react";

import { HexEditor, type SelectionSource } from "../components/hex/HexEditor";
import type { SelectionState } from "../components/hex/selection";
import { findNodeByPath } from "../components/tree/parseTree";
import { useWorkspace } from "../state/workspace";

export function HexPanel(_props: IDockviewPanelProps) {
  const {
    buffer,
    setBinary,
    hexSelection,
    setHexSelection,
    setHexSelectionFromPointer,
    tree,
    selectedTreePath,
  } = useWorkspace();

  // Derive the tree-driven highlight range from the currently-selected
  // tree node. The range is half-open in the JSON, so subtract one for
  // the inclusive [start, end] hex highlight.
  const highlight = useMemo<[number, number] | null>(() => {
    if (!tree || !selectedTreePath) return null;
    const node = findNodeByPath(tree, selectedTreePath);
    if (!node?.range) return null;
    const { startIndex, endIndex } = node.range;
    if (endIndex <= startIndex) return null;
    return [startIndex, endIndex - 1];
  }, [tree, selectedTreePath]);

  const onSelectionChange = useCallback(
    (s: SelectionState, source: SelectionSource) => {
      if (source === "pointer") {
        setHexSelectionFromPointer(s);
      } else {
        setHexSelection(s);
      }
    },
    [setHexSelection, setHexSelectionFromPointer],
  );

  const onFileLoad = useCallback(
    (_filename: string, bytes: Uint8Array) => {
      setBinary(bytes);
    },
    [setBinary],
  );

  return (
    <HexEditor
      buffer={buffer}
      selection={hexSelection}
      onSelectionChange={onSelectionChange}
      highlight={highlight}
      onFileLoad={onFileLoad}
    />
  );
}
