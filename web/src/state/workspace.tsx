import {
  type ReactNode,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { PieceTable } from "../buffer/pieceTable";
import type { SelectionState } from "../components/hex/selection";
import {
  type TreeNode,
  findDeepestNodeAtOffset,
  findNodeByPath,
  parseTreeJson,
} from "../components/tree/parseTree";
import { createLogger } from "../log";
import { EvalClient, type WorkerFactory } from "../rpc/evalClient";
import type { VFS } from "../vfs/types";
import {
  type Project,
  collectKsyFiles,
  ksyDisplayName,
  ksyPathToImportName,
} from "./project";

const log = createLogger("workspace");

const DEBOUNCE_MS = 250;

export interface WorkspaceState {
  client: EvalClient;
  project: Project;

  /** Source text of the *editor target* KSY (which may differ from
   *  the current KSY - see `editorKsyPath`). Mirrored from the VFS,
   *  empty when no KSY is open in the editor. */
  ksySource: string;
  /** Display name of the editor target, or null when none is open. */
  ksyName: string | null;
  /** Path of the KSY currently open in Monaco. May differ from the
   *  current parse target (`project.currentKsyPath`), which lets the
   *  user edit an imported helper without re-pointing the parse. */
  editorKsyPath: string | null;

  buffer: PieceTable;
  bufferVersion: number;

  tree: TreeNode | null;
  treeJson: string | null;
  error: string | null;
  loading: boolean;

  hexSelection: SelectionState;
  selectedTreePath: string | null;

  /** Set the current KSY by path (must already exist in the VFS).
   *  Pass null to clear the current selection. Only this affects the
   *  parse target; the editor stays put. */
  setCurrentKsyPath: (path: string | null) => void;
  /** Open a KSY in the editor. Doesn't change the parse target. Pass
   *  null to close the editor. */
  setEditorKsyPath: (path: string | null) => void;
  /** Replace the active project entirely - typically used to open a
   *  GitHub repo or switch between projects. Resets KSY/binary
   *  selections to whatever the new project carries. */
  setProject: (project: Project) => void;
  /** Edit the current KSY source (also writes through to the VFS). */
  setKsySource: (source: string) => void;
  /** Replace the binary buffer contents in one edit step. This is the
   *  external-drop path: it clears `currentBinaryPath` because the
   *  bytes no longer correspond to a file in the project. */
  setBinary: (bytes: Uint8Array) => void;
  /** Load a binary file from the project's VFS into the buffer and
   *  mark it as the currently-analyzed file. */
  openBinaryFromProject: (path: string) => Promise<void>;

  setHexSelection: (s: SelectionState) => void;
  setHexSelectionFromPointer: (s: SelectionState) => void;
  selectTreeNode: (path: string) => void;

  /** Tear down the current EvalClient (kills the worker) and spin up a
   *  fresh one. Use when the Go side has panicked or the worker has
   *  otherwise wedged. The next parse cycle will re-sync the project's
   *  KSY files to the new worker. */
  reloadWorker: () => void;
}

const WorkspaceContext = createContext<WorkspaceState | null>(null);

export function useWorkspace(): WorkspaceState {
  const ctx = useContext(WorkspaceContext);
  if (!ctx) throw new Error("useWorkspace called outside <WorkspaceProvider>");
  return ctx;
}

export interface WorkspaceProviderProps {
  /** The project to drive the workspace from. The provider owns the
   *  `currentKsyPath` / `currentBinaryPath` state internally and only
   *  reads `vfs` and `currentKsyPath` from the prop on first render. */
  initialProject: Project;
  /** Initial bytes for the buffer when no `currentBinaryPath` is set. */
  initialBinary: Uint8Array;
  workerFactory?: WorkerFactory;
  children: ReactNode;
}

export function WorkspaceProvider({
  initialProject,
  initialBinary,
  workerFactory,
  children,
}: WorkspaceProviderProps) {
  // Eval client lives in a ref so it can be swapped without forcing
  // the whole provider to re-render. `clientVersion` is a counter that
  // we *do* keep in state - bumping it triggers the parse effect to
  // re-run against the freshly-created client.
  const clientRef = useRef<EvalClient | null>(null);
  const [clientVersion, setClientVersion] = useState(0);
  if (clientRef.current === null) {
    log.debug("creating EvalClient");
    clientRef.current = new EvalClient(workerFactory);
  }

  const reloadWorker = useCallback(() => {
    log.info("reloadWorker - terminating old client, spawning fresh one");
    clientRef.current?.terminate();
    clientRef.current = new EvalClient(workerFactory);
    setClientVersion((v) => v + 1);
  }, [workerFactory]);

  const bufferRef = useRef<PieceTable | null>(null);
  if (bufferRef.current === null) {
    bufferRef.current = new PieceTable(initialBinary);
  }
  const buffer = bufferRef.current;

  // VFS used to be stable for the lifetime of the provider, but
  // opening a GitHub repo swaps it. Hold it in state so dependent
  // effects (parse, ksySource read, project panel listings) re-run.
  const [vfs, setVfs] = useState<VFS>(initialProject.vfs);
  const [currentKsyPath, setCurrentKsyPathState] = useState<string | null>(
    initialProject.currentKsyPath,
  );
  // The editor defaults to whatever KSY is current - for a fresh
  // project that's exactly what you want. The two diverge once the
  // user opens a different KSY via the project panel.
  const [editorKsyPath, setEditorKsyPathState] = useState<string | null>(
    initialProject.currentKsyPath,
  );
  const [currentBinaryPath, setCurrentBinaryPath] = useState<string | null>(
    initialProject.currentBinaryPath,
  );

  const project = useMemo<Project>(
    () => ({ vfs, currentKsyPath, currentBinaryPath }),
    [vfs, currentKsyPath, currentBinaryPath],
  );

  // Mirror the editor target's source into state (which may differ
  // from the current KSY - see editorKsyPath). Loading is async so we
  // keep an empty string until the first read completes.
  const [ksySource, setKsySourceState] = useState<string>("");

  useEffect(() => {
    if (editorKsyPath === null) {
      setKsySourceState("");
      return;
    }
    let cancelled = false;
    void vfs.readText(editorKsyPath).then(
      (text) => {
        if (cancelled) return;
        setKsySourceState(text);
      },
      (err) => {
        if (cancelled) return;
        log.error("failed to read KSY from VFS:", err);
        setKsySourceState("");
      },
    );
    return () => {
      cancelled = true;
    };
  }, [vfs, editorKsyPath]);

  const [bufferVersion, setBufferVersion] = useState<number>(buffer.version);
  const [treeJson, setTreeJson] = useState<string | null>(null);
  const [tree, setTree] = useState<TreeNode | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState<boolean>(true);

  const [hexSelection, setHexSelectionState] = useState<SelectionState>({
    anchor: 0,
    caret: 0,
    column: "hex",
  });
  const [selectedTreePath, setSelectedTreePath] = useState<string | null>(null);

  useEffect(() => {
    return buffer.subscribe(() => setBufferVersion(buffer.version));
  }, [buffer]);

  useEffect(() => {
    log.debug("parse effect mount", {
      ksy: currentKsyPath,
      bytes: buffer.length,
      bufferVersion,
    });
    if (currentKsyPath === null) {
      // No KSY selected -> no parse to run.
      setTree(null);
      setTreeJson(null);
      setError(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);

    const timer = setTimeout(() => {
      const client = clientRef.current!;
      void (async () => {
        try {
          await client.ready();
          if (cancelled) return;

          // Sync the VFS to the worker. Importing KSY files reference
          // each other by name, so we always push every KSY in the
          // project; redundant updates are cheap on the in-memory side.
          const ksyFiles = await collectKsyFiles(vfs);
          if (cancelled) return;
          for (const f of ksyFiles) {
            const name = ksyPathToImportName(f.path);
            await client.loadKsy(name, f.source);
            if (cancelled) return;
          }

          const rootName = ksyPathToImportName(currentKsyPath!);
          const bytes = buffer.toBytes();
          log.debug("parse", rootName, bytes.length, "bytes");
          const json = await client.parse(rootName, bytes);
          if (cancelled) return;
          log.info("parse complete:", json.length, "char tree");
          setTreeJson(json);
          setTree(parseTreeJson(json));
          setError(null);
        } catch (err) {
          log.error("parse failed:", err);
          if (cancelled) return;
          setError(err instanceof Error ? err.message : String(err));
          setTreeJson(null);
          setTree(null);
        } finally {
          if (!cancelled) setLoading(false);
        }
      })();
    }, DEBOUNCE_MS);

    return () => {
      log.debug("parse effect cleanup");
      cancelled = true;
      clearTimeout(timer);
    };
    // clientVersion is included so reloadWorker forces a parse against
    // the new client. (clientRef.current is read inside the effect; the
    // version counter is what makes React re-run it.)
  }, [vfs, currentKsyPath, ksySource, bufferVersion, buffer, clientVersion]);

  const setCurrentKsyPath = useCallback((path: string | null) => {
    setCurrentKsyPathState((prev) => (prev === path ? prev : path));
  }, []);

  const setEditorKsyPath = useCallback((path: string | null) => {
    setEditorKsyPathState((prev) => (prev === path ? prev : path));
  }, []);

  const setProject = useCallback((newProject: Project) => {
    log.info("setProject:", newProject.currentKsyPath ?? "(no KSY)");
    setVfs(newProject.vfs);
    setCurrentKsyPathState(newProject.currentKsyPath);
    setEditorKsyPathState(newProject.currentKsyPath);
    setCurrentBinaryPath(newProject.currentBinaryPath);
    setSelectedTreePath(null);
    setHexSelectionState({ anchor: 0, caret: 0, column: "hex" });
  }, []);

  const setKsySource = useCallback(
    (source: string) => {
      setKsySourceState((prev) => (prev === source ? prev : source));
      // Edits go to whichever file is open in the editor - not
      // necessarily the current parse target. This is the whole point
      // of splitting the two: edit a helper KSY while the parse keeps
      // targeting the root.
      if (editorKsyPath === null) return;
      void vfs.write(editorKsyPath, source).catch((err) => {
        log.error("VFS write failed:", err);
      });
    },
    [vfs, editorKsyPath],
  );

  // Internal helper used by both setBinary and openBinaryFromProject.
  const replaceBuffer = useCallback(
    (bytes: Uint8Array) => {
      if (buffer.length > 0) buffer.delete(0, buffer.length);
      if (bytes.length > 0) buffer.insert(0, bytes);
      setHexSelectionState({ anchor: 0, caret: 0, column: "hex" });
      setSelectedTreePath(null);
    },
    [buffer],
  );

  const setBinary = useCallback(
    (bytes: Uint8Array) => {
      // Load an external binary.
      setCurrentBinaryPath(null);
      replaceBuffer(bytes);
    },
    [replaceBuffer],
  );

  const openBinaryFromProject = useCallback(
    async (path: string) => {
      try {
        const bytes = await vfs.read(path);
        replaceBuffer(bytes);
        setCurrentBinaryPath(path);
      } catch (err) {
        log.error("openBinaryFromProject failed:", err);
      }
    },
    [vfs, replaceBuffer],
  );

  const setHexSelection = useCallback((s: SelectionState) => {
    setHexSelectionState(s);
  }, []);

  const setHexSelectionFromPointer = useCallback(
    (s: SelectionState) => {
      setHexSelectionState(s);
      if (!tree) {
        setSelectedTreePath(null);
        return;
      }
      const probe = Math.min(s.caret, buffer.length - 1);
      if (probe < 0) {
        setSelectedTreePath(null);
        return;
      }
      const node = findDeepestNodeAtOffset(tree, probe);
      setSelectedTreePath(node?.path ?? null);
    },
    [tree, buffer],
  );

  const selectTreeNode = useCallback(
    (path: string) => {
      setSelectedTreePath(path);
      if (!tree) return;
      const node = findNodeByPath(tree, path);
      if (!node || !node.range) return;
      const { startIndex, endIndex } = node.range;
      setHexSelectionState((prev) => ({
        anchor: startIndex,
        caret: endIndex,
        column: prev.column,
      }));
    },
    [tree],
  );

  const value = useMemo<WorkspaceState>(
    () => ({
      client: clientRef.current!,
      project,
      ksySource,
      ksyName: editorKsyPath ? ksyDisplayName(editorKsyPath) : null,
      editorKsyPath,
      buffer,
      bufferVersion,
      tree,
      treeJson,
      error,
      loading,
      hexSelection,
      selectedTreePath,
      setCurrentKsyPath,
      setEditorKsyPath,
      setProject,
      setKsySource,
      setBinary,
      openBinaryFromProject,
      setHexSelection,
      setHexSelectionFromPointer,
      selectTreeNode,
      reloadWorker,
    }),
    [
      project,
      ksySource,
      currentKsyPath,
      editorKsyPath,
      buffer,
      bufferVersion,
      tree,
      treeJson,
      error,
      loading,
      hexSelection,
      selectedTreePath,
      setCurrentKsyPath,
      setEditorKsyPath,
      setProject,
      setKsySource,
      setBinary,
      openBinaryFromProject,
      setHexSelection,
      setHexSelectionFromPointer,
      selectTreeNode,
      reloadWorker,
    ],
  );

  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  );
}
