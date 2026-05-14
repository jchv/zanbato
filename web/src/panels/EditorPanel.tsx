import type { IDockviewPanelProps } from "dockview-react";

import { useWorkspace } from "../state/workspace";
import { MonacoEditor } from "./MonacoEditor";

export function EditorPanel(_props: IDockviewPanelProps) {
  const { ksySource, ksyName, setKsySource } = useWorkspace();
  if (ksyName === null) {
    return (
      <div className="editor-placeholder">
        Select a KSY file in the project tree to start editing.
      </div>
    );
  }
  return (
    <MonacoEditor
      value={ksySource}
      language="yaml"
      path={`${ksyName}.ksy`}
      onChange={setKsySource}
    />
  );
}
