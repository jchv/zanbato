import * as ContextMenu from "@radix-ui/react-context-menu";
import type { IDockviewPanelProps } from "dockview-react";
import {
  CloudDownload,
  File as FileIcon,
  FilePlus,
  FileText,
  Folder as FolderIcon,
  FolderOpen,
  FolderPlus,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { memo, useEffect, useState } from "react";

import { openGitHubProject } from "../state/githubProject";
import { KSY_EXTENSION } from "../state/project";
import { useWorkspace } from "../state/workspace";
import { type VFS, type VFSEntry, dirname } from "../vfs/types";
import "./project.css";

const ROOT_LABEL = "project";

export function ProjectPanel(_props: IDockviewPanelProps) {
  const {
    project,
    editorKsyPath,
    setCurrentKsyPath,
    setEditorKsyPath,
    openBinaryFromProject,
    setProject,
    reloadWorker,
  } = useWorkspace();
  const { vfs, currentKsyPath, currentBinaryPath } = project;

  const [refreshKey, setRefreshKey] = useState(0);
  useEffect(() => {
    return vfs.subscribe(() => setRefreshKey((k) => k + 1));
  }, [vfs]);
  const [loadingRepo, setLoadingRepo] = useState(false);

  const handleSelectKsy = (path: string) => {
    setEditorKsyPath(path);
    if (currentKsyPath === null) setCurrentKsyPath(path);
  };
  const handleMakeCurrent = (path: string) => {
    setCurrentKsyPath(path);
  };
  const handleSelectBinary = (path: string) => {
    void openBinaryFromProject(path);
  };

  const handleOpenGitHub = async () => {
    const spec = window.prompt(
      "GitHub repo to open:\n\nAccepts owner/repo, owner/repo@ref, or a github.com URL.",
      "kaitai-io/kaitai_struct_formats",
    );
    if (!spec) return;
    setLoadingRepo(true);
    try {
      const newProject = await openGitHubProject(spec);
      setProject(newProject);
    } catch (err) {
      window.alert(`Failed to open repo: ${(err as Error).message}`);
    } finally {
      setLoadingRepo(false);
    }
  };

  return (
    <div className="project-panel" role="tree">
      <DirectoryNode
        vfs={vfs}
        path="/"
        label={ROOT_LABEL}
        depth={0}
        forceExpanded
        currentKsyPath={currentKsyPath}
        editorKsyPath={editorKsyPath}
        currentBinaryPath={currentBinaryPath}
        onSelectKsy={handleSelectKsy}
        onMakeCurrent={handleMakeCurrent}
        onSelectBinary={handleSelectBinary}
        onOpenGitHub={handleOpenGitHub}
        onReloadWorker={reloadWorker}
        refreshKey={refreshKey}
      />
      {loadingRepo && (
        <div className="project-loading">Fetching repository tree...</div>
      )}
    </div>
  );
}

interface DirectoryNodeProps {
  vfs: VFS;
  path: string;
  label: string;
  depth: number;
  forceExpanded?: boolean;
  currentKsyPath: string | null;
  editorKsyPath: string | null;
  currentBinaryPath: string | null;
  onSelectKsy: (path: string) => void;
  onMakeCurrent: (path: string) => void;
  onSelectBinary: (path: string) => void;
  onOpenGitHub?: () => void;
  onReloadWorker?: () => void;
  refreshKey: number;
}

const DirectoryNode = memo(function DirectoryNode({
  vfs,
  path,
  label,
  depth,
  forceExpanded = false,
  currentKsyPath,
  editorKsyPath,
  currentBinaryPath,
  onSelectKsy,
  onMakeCurrent,
  onSelectBinary,
  onOpenGitHub,
  onReloadWorker,
  refreshKey,
}: DirectoryNodeProps) {
  const [expanded, setExpanded] = useState(forceExpanded || depth === 0);
  const [entries, setEntries] = useState<VFSEntry[]>([]);
  const [dragOver, setDragOver] = useState(false);

  useEffect(() => {
    if (!expanded) return;
    let cancelled = false;
    void vfs.list(path).then((list) => {
      if (!cancelled) setEntries(list);
    });
    return () => {
      cancelled = true;
    };
  }, [vfs, path, refreshKey, expanded]);

  const onNewFile = async () => {
    const name = window.prompt(`New file in ${path === "/" ? "/" : path}:`);
    if (!name) return;
    const target = joinPath(path, name);
    try {
      const initial = target.endsWith(KSY_EXTENSION)
        ? "meta:\n  id: new\nseq: []\n"
        : new Uint8Array();
      await vfs.write(target, initial);
    } catch (err) {
      window.alert(`Failed to create ${target}: ${(err as Error).message}`);
    }
  };

  const onNewFolder = async () => {
    const name = window.prompt(`New folder in ${path === "/" ? "/" : path}:`);
    if (!name) return;
    const target = joinPath(path, name);
    try {
      await vfs.mkdir(target);
    } catch (err) {
      window.alert(`Failed to create ${target}: ${(err as Error).message}`);
    }
  };

  const onDeleteSelf = async () => {
    if (path === "/") return; // can't delete the root
    if (!window.confirm(`Delete ${path} and everything inside?`)) return;
    await vfs.delete(path);
  };

  // File drag-and-drop - drop external files into this directory.
  const onDragEnter = (e: React.DragEvent) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    setDragOver(true);
  };
  const onDragOver = (e: React.DragEvent) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
  };
  const onDragLeave = (e: React.DragEvent) => {
    if (!hasFiles(e)) return;
    setDragOver(false);
  };
  const onDrop = async (e: React.DragEvent) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    e.stopPropagation();
    setDragOver(false);
    for (const file of Array.from(e.dataTransfer.files)) {
      const target = joinPath(path, file.name);
      const buf = await file.arrayBuffer();
      await vfs.write(target, new Uint8Array(buf));
    }
  };

  const rowClasses = [
    "project-row",
    "project-row--dir",
    dragOver ? "project-row--drop-target" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <>
      <ContextMenu.Root>
        <ContextMenu.Trigger asChild>
          <div
            className={rowClasses}
            style={{ paddingLeft: indentPx(depth) }}
            onClick={() => !forceExpanded && setExpanded((v) => !v)}
            onDragEnter={onDragEnter}
            onDragOver={onDragOver}
            onDragLeave={onDragLeave}
            onDrop={onDrop}
            role="treeitem"
            aria-expanded={expanded}
          >
            <span className="project-disclosure">
              {forceExpanded ? null : expanded ? "▾" : "▸"}
            </span>
            <span className="project-icon">
              {expanded ? <FolderOpen size={14} /> : <FolderIcon size={14} />}
            </span>
            <span className="project-name">{label}</span>
          </div>
        </ContextMenu.Trigger>
        <ContextMenu.Portal>
          <ContextMenu.Content className="ctx-menu">
            <ContextMenu.Item className="ctx-menu-item" onSelect={onNewFile}>
              <FilePlus size={13} />
              New File...
            </ContextMenu.Item>
            <ContextMenu.Item className="ctx-menu-item" onSelect={onNewFolder}>
              <FolderPlus size={13} />
              New Folder...
            </ContextMenu.Item>
            {(onOpenGitHub || onReloadWorker) && (
              <ContextMenu.Separator className="ctx-menu-separator" />
            )}
            {onOpenGitHub && (
              <ContextMenu.Item
                className="ctx-menu-item"
                onSelect={onOpenGitHub}
              >
                <CloudDownload size={13} />
                Open GitHub Repo...
              </ContextMenu.Item>
            )}
            {onReloadWorker && (
              <ContextMenu.Item
                className="ctx-menu-item"
                onSelect={onReloadWorker}
              >
                <RefreshCw size={13} />
                Reload Worker
              </ContextMenu.Item>
            )}
            {path !== "/" && (
              <>
                <ContextMenu.Separator className="ctx-menu-separator" />
                <ContextMenu.Item
                  className="ctx-menu-item ctx-menu-item--destructive"
                  onSelect={onDeleteSelf}
                >
                  <Trash2 size={13} />
                  Delete Folder
                </ContextMenu.Item>
              </>
            )}
          </ContextMenu.Content>
        </ContextMenu.Portal>
      </ContextMenu.Root>
      {expanded &&
        entries.map((entry) =>
          entry.type === "directory" ? (
            <DirectoryNode
              key={entry.path}
              vfs={vfs}
              path={entry.path}
              label={entry.name}
              depth={depth + 1}
              currentKsyPath={currentKsyPath}
              editorKsyPath={editorKsyPath}
              currentBinaryPath={currentBinaryPath}
              onSelectKsy={onSelectKsy}
              onMakeCurrent={onMakeCurrent}
              onSelectBinary={onSelectBinary}
              refreshKey={refreshKey}
            />
          ) : (
            <FileRow
              key={entry.path}
              vfs={vfs}
              entry={entry}
              depth={depth + 1}
              isCurrentKsy={entry.path === currentKsyPath}
              isEditorKsy={entry.path === editorKsyPath}
              isCurrentBinary={entry.path === currentBinaryPath}
              onSelectKsy={onSelectKsy}
              onMakeCurrent={onMakeCurrent}
              onSelectBinary={onSelectBinary}
            />
          ),
        )}
    </>
  );
});

interface FileRowProps {
  vfs: VFS;
  entry: VFSEntry;
  depth: number;
  isCurrentKsy: boolean;
  isEditorKsy: boolean;
  isCurrentBinary: boolean;
  onSelectKsy: (path: string) => void;
  onMakeCurrent: (path: string) => void;
  onSelectBinary: (path: string) => void;
}

function FileRow({
  vfs,
  entry,
  depth,
  isCurrentKsy,
  isEditorKsy,
  isCurrentBinary,
  onSelectKsy,
  onMakeCurrent,
  onSelectBinary,
}: FileRowProps) {
  const isKsy = entry.path.endsWith(KSY_EXTENSION);

  const onClick = () => {
    if (isKsy) onSelectKsy(entry.path);
    else onSelectBinary(entry.path);
  };

  const onDelete = async () => {
    if (!window.confirm(`Delete ${entry.path}?`)) return;
    await vfs.delete(entry.path);
  };

  const className = [
    "project-row",
    "project-row--file",
    isKsy ? "project-row--ksy" : "project-row--bin",
    isCurrentKsy ? "project-row--current-ksy" : "",
    isEditorKsy && !isCurrentKsy ? "project-row--editor-ksy" : "",
    isCurrentBinary ? "project-row--current-bin" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <ContextMenu.Root>
      <ContextMenu.Trigger asChild>
        <div
          className={className}
          style={{ paddingLeft: indentPx(depth) }}
          onClick={onClick}
          role="treeitem"
          aria-selected={isCurrentKsy || isCurrentBinary}
        >
          <span className="project-disclosure project-disclosure--leaf" />
          <span className="project-icon">
            {isKsy ? <FileText size={14} /> : <FileIcon size={14} />}
          </span>
          <span className="project-name">{entry.name}</span>
        </div>
      </ContextMenu.Trigger>
      <ContextMenu.Portal>
        <ContextMenu.Content className="ctx-menu">
          {isKsy && (
            <>
              <ContextMenu.Item
                className="ctx-menu-item"
                onSelect={() => onSelectKsy(entry.path)}
              >
                Open in Editor
              </ContextMenu.Item>
              <ContextMenu.Item
                className="ctx-menu-item"
                onSelect={() => onMakeCurrent(entry.path)}
                disabled={isCurrentKsy}
              >
                Make Current
              </ContextMenu.Item>
            </>
          )}
          {!isKsy && (
            <ContextMenu.Item
              className="ctx-menu-item"
              onSelect={() => onSelectBinary(entry.path)}
            >
              Open as Binary
            </ContextMenu.Item>
          )}
          <ContextMenu.Separator className="ctx-menu-separator" />
          <ContextMenu.Item
            className="ctx-menu-item ctx-menu-item--destructive"
            onSelect={onDelete}
          >
            <Trash2 size={13} />
            Delete
          </ContextMenu.Item>
        </ContextMenu.Content>
      </ContextMenu.Portal>
    </ContextMenu.Root>
  );
}

function joinPath(dir: string, name: string): string {
  const clean = name.replace(/^\/+|\/+$/g, "");
  if (!clean) return dir;
  return dir === "/" ? "/" + clean : dir + "/" + clean;
}

function hasFiles(e: React.DragEvent): boolean {
  const types = e.dataTransfer.types;
  for (let i = 0; i < types.length; i++) {
    if (types[i] === "Files") return true;
  }
  return false;
}

function indentPx(depth: number): string {
  return `${depth === 0 ? 4 : depth * 14 + 4}px`;
}

export { dirname };
