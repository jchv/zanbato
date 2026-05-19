import { createLogger } from "../log";
import type {
  EvalRequest,
  EvalResponse,
  KsyFile,
  WorkerEvent,
} from "./protocol";

const log = createLogger("rpc");

export interface WorkerLike {
  postMessage(message: unknown, transfer?: Transferable[]): void;
  terminate(): void;
  onmessage: ((this: WorkerLike, ev: MessageEvent) => void) | null;
  onerror: ((this: WorkerLike, ev: ErrorEvent) => void) | null;
}

export type WorkerFactory = () => WorkerLike;

interface Pending {
  resolve: (value: EvalResponse) => void;
  reject: (reason: Error) => void;
}

export class EvalClient {
  private worker: WorkerLike;
  private nextId = 1;
  private pending = new Map<number, Pending>();
  private readyPromise: Promise<void>;
  private resolveReady!: () => void;
  private rejectReady!: (err: Error) => void;
  private terminated = false;

  constructor(factory: WorkerFactory = defaultWorkerFactory) {
    this.worker = factory();
    this.worker.onmessage = (e) => this.onMessage(e.data as WorkerEvent);
    this.worker.onerror = (e) => {
      const err = new Error(`worker error: ${e.message ?? "unknown"}`);
      this.rejectReady?.(err);
      this.failAllPending(err);
    };
    this.readyPromise = new Promise<void>((resolve, reject) => {
      this.resolveReady = resolve;
      this.rejectReady = reject;
    });
  }

  /** Resolves once the worker has finished booting (wasm instantiated). */
  ready(): Promise<void> {
    return this.readyPromise;
  }

  async loadKsys(files: KsyFile[]): Promise<void> {
    const r = await this.call({ type: "loadKsys", files });
    return unwrap(r);
  }

  async parse(rootName: string, data: Uint8Array): Promise<string> {
    const r = await this.call({ type: "parse", rootName, data });
    if (!r.ok) throw new Error(r.error);
    if (r.tree === undefined) {
      throw new Error("parse: response missing tree");
    }
    return r.tree;
  }

  terminate(): void {
    if (this.terminated) return;
    this.terminated = true;
    this.worker.terminate();
    this.failAllPending(new Error("worker terminated"));
  }

  private call(request: EvalRequest): Promise<EvalResponse> {
    if (this.terminated) {
      return Promise.reject(new Error("worker terminated"));
    }
    return new Promise<EvalResponse>((resolve, reject) => {
      const id = this.nextId++;
      this.pending.set(id, { resolve, reject });
      log.debug("->", id, request.type);
      this.worker.postMessage({ id, request });
    });
  }

  private onMessage(msg: WorkerEvent): void {
    if ("type" in msg) {
      if (msg.type === "ready") {
        // Info-level so the main-thread side of "ready" is visible
        // alongside the worker's "[worker.boot] ready" log without
        // requiring the user to enable the debug filter.
        log.info("worker reports ready");
        this.resolveReady();
      } else if (msg.type === "ready-error") {
        // Worker boot failed before the wasm could register its API.
        // Reject the ready promise so callers see the failure instead
        // of waiting forever.
        log.error("worker reports ready-error:", msg.error);
        this.rejectReady(new Error(`worker boot failed: ${msg.error}`));
        this.failAllPending(new Error(`worker boot failed: ${msg.error}`));
      }
      return;
    }
    if (!("id" in msg)) return;
    const entry = this.pending.get(msg.id);
    if (!entry) {
      log.debug("<- stale/duplicate response", msg.id);
      return;
    }
    this.pending.delete(msg.id);
    log.debug(
      "<-",
      msg.id,
      msg.response.ok ? "ok" : `err: ${msg.response.error}`,
    );
    entry.resolve(msg.response);
  }

  private failAllPending(err: Error): void {
    for (const { reject } of this.pending.values()) reject(err);
    this.pending.clear();
  }
}

function unwrap(r: EvalResponse): void {
  if (!r.ok) throw new Error(r.error);
}

function defaultWorkerFactory(): WorkerLike {
  return new Worker(new URL("../workers/eval.worker.ts", import.meta.url), {
    type: "module",
  }) as unknown as WorkerLike;
}
