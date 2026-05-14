import type { IDockviewPanelProps } from "dockview-react";

import { TreeView } from "../components/tree/TreeView";
import { useWorkspace } from "../state/workspace";

export function TreePanel(_props: IDockviewPanelProps) {
  const {
    tree,
    error,
    loading,
    selectedTreePath,
    selectTreeNode,
    reloadWorker,
  } = useWorkspace();

  if (error) {
    return (
      <div className="tree-panel-error-wrap">
        <pre className="tree-panel-error">{error}</pre>
        <button
          type="button"
          className="tree-panel-reload"
          onClick={reloadWorker}
        >
          Reload Worker
        </button>
      </div>
    );
  }
  if (!tree) {
    const msg = loading
      ? "Booting wasm and parsing..."
      : "Select a KSY in the project tree to see the parse result.";
    return <div className="tree-panel-status">{msg}</div>;
  }
  return (
    <TreeView
      root={tree}
      selectedPath={selectedTreePath}
      onSelect={selectTreeNode}
    />
  );
}
