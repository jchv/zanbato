import { describe, expect, it } from "vitest";

import { EvalClient, type WorkerLike } from "./evalClient";
import type { RpcRequest, WorkerEvent } from "./protocol";

class FakeWorker implements WorkerLike {
  onmessage: ((this: WorkerLike, ev: MessageEvent) => void) | null = null;
  onerror: ((this: WorkerLike, ev: ErrorEvent) => void) | null = null;
  sent: RpcRequest[] = [];
  terminated = false;

  postMessage(message: unknown): void {
    this.sent.push(message as RpcRequest);
  }

  terminate(): void {
    this.terminated = true;
  }

  emit(event: WorkerEvent): void {
    this.onmessage?.call(this, { data: event } as MessageEvent);
  }

  emitError(message: string): void {
    this.onerror?.call(this, { message } as ErrorEvent);
  }

  reply(
    n: number,
    response:
      | WorkerEvent
      | (RpcRequest extends { id: infer I } ? Partial<{ id: I }> : never),
  ): void {
    const sent = this.sent[n];
    if (!sent) throw new Error(`no sent message at index ${n}`);
    this.emit({
      id: sent.id,
      response: response as never,
    } as WorkerEvent);
  }
}

function makeClient(): { client: EvalClient; worker: FakeWorker } {
  let captured!: FakeWorker;
  const client = new EvalClient(() => {
    captured = new FakeWorker();
    return captured;
  });
  return { client, worker: captured };
}

describe("EvalClient", () => {
  describe("ready", () => {
    it("resolves on a ready event from the worker", async () => {
      const { client, worker } = makeClient();
      let resolved = false;
      const p = client.ready().then(() => {
        resolved = true;
      });
      expect(resolved).toBe(false);
      worker.emit({ type: "ready" });
      await p;
      expect(resolved).toBe(true);
    });

    it("rejects on a worker error", async () => {
      const { client, worker } = makeClient();
      const p = client.ready();
      worker.emitError("boom");
      await expect(p).rejects.toThrow(/boom/);
    });
  });

  describe("request correlation", () => {
    it("returns the response matched by id", async () => {
      const { client, worker } = makeClient();
      const p = client.loadKsys([{ name: "foo", source: "src" }]);
      expect(worker.sent).toHaveLength(1);
      const req = worker.sent[0]!;
      expect(req.request).toEqual({
        type: "loadKsys",
        files: [{ name: "foo", source: "src" }],
      });
      worker.emit({ id: req.id, response: { ok: true } });
      await expect(p).resolves.toBeUndefined();
    });

    it("handles concurrent requests properly", async () => {
      const { client, worker } = makeClient();
      const p1 = client.loadKsys([{ name: "a", source: "src-a" }]);
      const p2 = client.loadKsys([{ name: "b", source: "src-b" }]);
      const p3 = client.parse("a", new Uint8Array([1, 2, 3]));

      expect(worker.sent).toHaveLength(3);
      // Reply out of order.
      worker.emit({
        id: worker.sent[2]!.id,
        response: { ok: true, tree: "{tree3}" },
      });
      worker.emit({ id: worker.sent[0]!.id, response: { ok: true } });
      worker.emit({ id: worker.sent[1]!.id, response: { ok: true } });

      await expect(p1).resolves.toBeUndefined();
      await expect(p2).resolves.toBeUndefined();
      await expect(p3).resolves.toBe("{tree3}");
    });

    it("ignores stale or duplicate responses", async () => {
      const { client, worker } = makeClient();
      const p = client.loadKsys([{ name: "foo", source: "src" }]);
      const id = worker.sent[0]!.id;
      worker.emit({ id, response: { ok: true } });
      // Duplicate - should be silently ignored.
      worker.emit({ id, response: { ok: false, error: "should not reach" } });
      await expect(p).resolves.toBeUndefined();
    });

    it("ignores responses for unknown ids", () => {
      const { worker } = makeClient();
      // No requests yet; emitting any response should be a no-op.
      expect(() =>
        worker.emit({ id: 999, response: { ok: true } }),
      ).not.toThrow();
    });
  });

  describe("error paths", () => {
    it("rejects loadKsys with the worker-supplied error message", async () => {
      const { client, worker } = makeClient();
      const p = client.loadKsys([{ name: "bad", source: "src" }]);
      worker.emit({
        id: worker.sent[0]!.id,
        response: { ok: false, error: "parse failed" },
      });
      await expect(p).rejects.toThrow(/parse failed/);
    });

    it("rejects parse with the error and never returns a tree", async () => {
      const { client, worker } = makeClient();
      const p = client.parse("x", new Uint8Array());
      worker.emit({
        id: worker.sent[0]!.id,
        response: { ok: false, error: "no such root" },
      });
      await expect(p).rejects.toThrow(/no such root/);
    });

    it("rejects parse with a clear error if response omits tree", async () => {
      const { client, worker } = makeClient();
      const p = client.parse("x", new Uint8Array());
      worker.emit({ id: worker.sent[0]!.id, response: { ok: true } });
      await expect(p).rejects.toThrow(/missing tree/);
    });
  });

  describe("termination", () => {
    it("rejects all pending requests on terminate", async () => {
      const { client, worker } = makeClient();
      const p1 = client.loadKsys([{ name: "a", source: "src" }]);
      const p2 = client.parse("a", new Uint8Array());
      client.terminate();
      await expect(p1).rejects.toThrow(/terminated/);
      await expect(p2).rejects.toThrow(/terminated/);
      expect(worker.terminated).toBe(true);
    });

    it("rejects new requests after terminate", async () => {
      const { client } = makeClient();
      client.terminate();
      await expect(
        client.loadKsys([{ name: "a", source: "src" }]),
      ).rejects.toThrow(/terminated/);
    });

    it("is idempotent", () => {
      const { client, worker } = makeClient();
      client.terminate();
      client.terminate();
      expect(worker.terminated).toBe(true);
    });
  });

  describe("id allocation", () => {
    it("assigns monotonically increasing ids", () => {
      const { client, worker } = makeClient();
      void client.loadKsys([{ name: "a", source: "" }]);
      void client.loadKsys([{ name: "b", source: "" }]);
      void client.loadKsys([{ name: "c", source: "" }]);
      expect(worker.sent.map((m) => m.id)).toEqual([1, 2, 3]);
    });
  });
});
