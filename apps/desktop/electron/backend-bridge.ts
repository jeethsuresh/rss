import { Readable, Writable } from "node:stream";

export type BackendEvent = { event: string; payload: unknown };

type Pending = {
  resolve: (value: unknown) => void;
  reject: (err: Error) => void;
};

export class BackendBridge {
  private nextId = 1;
  private pending = new Map<string, Pending>();
  private buffer = "";
  private eventHandlers = new Set<(event: BackendEvent) => void>();

  constructor(
    private readonly stdin: Writable,
    stdout: Readable,
  ) {
    stdout.setEncoding("utf8");
    stdout.on("data", (chunk: string) => this.onData(chunk));
    stdout.on("end", () => this.rejectAll(new Error("Backend stdout closed")));
  }

  onEvent(handler: (event: BackendEvent) => void): () => void {
    this.eventHandlers.add(handler);
    return () => this.eventHandlers.delete(handler);
  }

  request(method: string, params: unknown): Promise<unknown> {
    const id = String(this.nextId++);
    const payload = JSON.stringify({ id, method, params }) + "\n";
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.stdin.write(payload, (err) => {
        if (err) {
          this.pending.delete(id);
          reject(err);
        }
      });
    });
  }

  private onData(chunk: string) {
    this.buffer += chunk;
    let idx: number;
    while ((idx = this.buffer.indexOf("\n")) >= 0) {
      const line = this.buffer.slice(0, idx).trim();
      this.buffer = this.buffer.slice(idx + 1);
      if (!line) continue;
      try {
        const msg = JSON.parse(line) as {
          id?: string;
          result?: unknown;
          error?: { code: string; message: string };
          event?: string;
          payload?: unknown;
        };
        if (msg.event) {
          const ev = { event: msg.event, payload: msg.payload };
          for (const h of this.eventHandlers) h(ev);
          continue;
        }
        if (!msg.id) continue;
        const pending = this.pending.get(msg.id);
        if (!pending) continue;
        this.pending.delete(msg.id);
        if (msg.error) {
          pending.reject(new Error(`${msg.error.code}: ${msg.error.message}`));
        } else {
          pending.resolve(msg.result);
        }
      } catch (err) {
        console.error("Failed to parse backend message", err, line);
      }
    }
  }

  private rejectAll(err: Error) {
    for (const [, p] of this.pending) p.reject(err);
    this.pending.clear();
  }
}
