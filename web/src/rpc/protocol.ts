export interface LoadKsyRequest {
  type: "loadKsy";
  name: string;
  source: string;
}

export interface ParseRequest {
  type: "parse";
  rootName: string;
  data: Uint8Array;
}

export interface ClearVfsRequest {
  type: "clearVfs";
}

export type EvalRequest = LoadKsyRequest | ParseRequest | ClearVfsRequest;

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
