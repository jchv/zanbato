/// <reference lib="webworker" />
import { createLogger } from "../log";
import type { EvalRequest, EvalResponse, RpcRequest } from "../rpc/protocol";

const log = createLogger("worker.boot");

interface ZanbatoAPI {
  loadKsy: (name: string, source: string) => EvalResponse;
  parse: (rootName: string, data: Uint8Array) => EvalResponse;
  clearVfs: () => EvalResponse;
}

declare const self: DedicatedWorkerGlobalScope;

let api: ZanbatoAPI | undefined;
const queue: MessageEvent<RpcRequest>[] = [];

self.addEventListener("message", (e: MessageEvent<RpcRequest>) => {
  if (!api) {
    log.debug("queueing pre-boot message", e.data?.request?.type);
    queue.push(e);
    return;
  }
  handle(e.data);
});

function handle(msg: RpcRequest): void {
  log.debug("dispatching", msg.request.type);
  let response: EvalResponse;
  try {
    response = dispatch(msg.request);
  } catch (err) {
    response = { ok: false, error: errorMessage(err) };
  }
  self.postMessage({ id: msg.id, response });
}

function dispatch(req: EvalRequest): EvalResponse {
  if (!api) {
    return { ok: false, error: "wasm not initialized" };
  }
  switch (req.type) {
    case "loadKsy":
      return api.loadKsy(req.name, req.source);
    case "parse":
      return api.parse(req.rootName, req.data);
    case "clearVfs":
      return api.clearVfs();
  }
}

async function boot(): Promise<void> {
  log.debug("boot starting");
  log.debug("fetching wasm_exec.js");
  const execResp = await fetch(`${import.meta.env.BASE_URL}wasm_exec.js`);
  if (!execResp.ok) {
    throw new Error(`wasm_exec.js fetch failed: HTTP ${execResp.status}`);
  }
  const wasmExecSrc = await execResp.text();
  log.debug("wasm_exec.js fetched", wasmExecSrc.length, "bytes");
  // Indirect eval keeps assignments to `globalThis.*` visible to us.
  (0, eval)(wasmExecSrc);
  log.debug(
    "wasm_exec.js evaluated; globalThis.Go =",
    typeof (self as unknown as { Go?: unknown }).Go,
  );

  const GoCtor = (self as unknown as { Go?: new () => GoInstance }).Go;
  if (typeof GoCtor !== "function") {
    throw new Error("wasm_exec.js did not register globalThis.Go");
  }
  const go = new GoCtor();

  log.debug("fetching zanbato.wasm");
  const { instance } = await WebAssembly.instantiateStreaming(
    fetch(`${import.meta.env.BASE_URL}zanbato.wasm`),
    go.importObject,
  );
  log.debug("wasm instantiated; starting Go runtime");

  // main() blocks on `select{}` so this never resolves. The Go-side global
  // is registered synchronously inside main() before the block.
  void go.run(instance);

  // Yield to let main() execute and register `zanbato` on globalThis.
  for (let i = 0; i < 1000 && api === undefined; i++) {
    const candidate = (self as unknown as { zanbato?: ZanbatoAPI }).zanbato;
    if (candidate) {
      api = candidate;
      log.debug("zanbato API registered after", i, "ticks");
      break;
    }
    await new Promise((r) => setTimeout(r, 1));
  }
  if (!api) {
    throw new Error("zanbato wasm did not register API within 1s");
  }

  // Drain anything that arrived during boot.
  if (queue.length > 0) {
    log.debug("draining", queue.length, "queued messages");
    for (const e of queue) handle(e.data);
    queue.length = 0;
  }

  log.info("ready");
  self.postMessage({ type: "ready" });
}

interface GoInstance {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}

void boot().catch((err) => {
  log.error("boot failed:", err);
  self.postMessage({
    type: "ready-error",
    error: errorMessage(err),
  });
});
