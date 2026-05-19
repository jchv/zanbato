export interface KsyFile {
  /** Slash-separated import name without `.ksy` extension, e.g. "foo" or
   *  "sub/bar". */
  name: string;
  source: string;
}

export interface LoadKsysRequest {
  type: "loadKsys";
  files: KsyFile[];
}

export interface ParseRequest {
  type: "parse";
  rootName: string;
  data: Uint8Array;
}

export type EvalRequest = LoadKsysRequest | ParseRequest;

export type EvalResponse =
  | { ok: true; tree?: string }
  | { ok: false; error: string };

export interface RpcRequest {
  id: number;
  request: EvalRequest;
}

export interface RpcResponse {
  id: number;
  response: EvalResponse;
}

export interface WorkerReadyEvent {
  type: "ready";
}

export interface WorkerReadyErrorEvent {
  type: "ready-error";
  error: string;
}

export type WorkerEvent =
  | RpcResponse
  | WorkerReadyEvent
  | WorkerReadyErrorEvent;
