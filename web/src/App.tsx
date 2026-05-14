import {
  DockviewReact,
  type DockviewReadyEvent,
  type IDockviewPanelProps,
} from "dockview-react";
import "dockview-react/dist/styles/dockview.css";

import "./app.css";
import { withErrorBoundary } from "./components/ErrorBoundary";
import { EditorPanel } from "./panels/EditorPanel";
import { HexPanel } from "./panels/HexPanel";
import { ProjectPanel } from "./panels/ProjectPanel";
import { TreePanel } from "./panels/TreePanel";
import type { Project } from "./state/project";
import { WorkspaceProvider } from "./state/workspace";
import { InMemoryVFS } from "./vfs/memory";

const components: Record<
  string,
  React.FunctionComponent<IDockviewPanelProps>
> = {
  project: withErrorBoundary(ProjectPanel, "Project panel crashed"),
  editor: withErrorBoundary(EditorPanel, "Editor panel crashed"),
  hex: withErrorBoundary(HexPanel, "Hex panel crashed"),
  tree: withErrorBoundary(TreePanel, "Tree panel crashed"),
};

function onReady(event: DockviewReadyEvent) {
  event.api.addPanel({
    id: "project",
    component: "project",
    title: "Project",
    initialWidth: 200,
  });
  event.api.addPanel({
    id: "editor",
    component: "editor",
    title: "Editor",
    position: { referencePanel: "project", direction: "right" },
  });
  event.api.addPanel({
    id: "hex",
    component: "hex",
    title: "Hex",
    position: { referencePanel: "editor", direction: "below" },
  });
  event.api.addPanel({
    id: "tree",
    component: "tree",
    title: "Tree",
    position: { referencePanel: "editor", direction: "right" },
    initialWidth: 200,
  });
}

const SEED_KSY_SOURCE = [
  "meta:",
  "  id: smoke",
  "seq:",
  "  - id: magic",
  "    contents: [0xCA, 0xFE]",
  "  - id: count",
  "    type: u2le",
  "  - id: items",
  "    type: u1",
  "    repeat: expr",
  "    repeat-expr: count",
  "",
].join("\n");

const SEED_BINARY = new Uint8Array([0xca, 0xfe, 0x04, 0x00, 1, 2, 3, 4]);

const seedVfs = InMemoryVFS.fromSeed({
  "/hello.ksy": SEED_KSY_SOURCE,
});

const seedProject: Project = {
  vfs: seedVfs,
  currentKsyPath: "/hello.ksy",
  currentBinaryPath: null,
};

export function App() {
  return (
    <WorkspaceProvider initialProject={seedProject} initialBinary={SEED_BINARY}>
      <DockviewReact
        className="dockview-theme-abyss"
        components={components}
        onReady={onReady}
      />
    </WorkspaceProvider>
  );
}
