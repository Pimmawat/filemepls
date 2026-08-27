import type { FileMeta } from "./protocol";
import type { SendSocket } from "./socket";

export const CHUNK_SIZE = 256 * 1024;
// Pause sending once this many bytes are buffered but not yet handed to
// the network, and resume on the channel's "bufferedamountlow" event —
// without this, a fast sender on a slow/congested DataChannel would queue
// the entire file in memory before any of it actually goes out.
const BUFFERED_AMOUNT_LOW_THRESHOLD = 1 << 20; // 1MB

// File reads are batched into blocks this large before being split into
// CHUNK_SIZE messages. `file.slice(...).arrayBuffer()` isn't free — for a
// File backed by an actual on-disk file, each call involves the browser
// actually reading those bytes (real I/O, possibly proxied across a
// process boundary depending on the browser). Calling it once per 16KB
// chunk (6,400 times for a 100MB file) means 6,400 separate reads; calling
// it once per 4MB block means ~25. The CHUNK_SIZE sub-slicing within an
// already-in-memory block is a synchronous, free ArrayBuffer.slice — no
// further I/O — so this keeps individual DataChannel messages small
// without paying the I/O cost per message.
const READ_BLOCK_SIZE = 4 * 1024 * 1024; // 4MB

// Minimum gap between onProgress calls. A naive implementation calls
// onProgress on every 16KB chunk — for a 100MB file that's ~6,400 calls,
// each one triggering a React state update + re-render. That re-render
// work competes for the same main thread the chunking loop and the
// DataChannel's own message handling need, and was the actual bottleneck
// behind large transfers feeling far slower than the network/WebRTC
// layer could otherwise do — not anything in WebRTC itself. Throttling
// to ~10 updates/sec keeps the progress bar smooth without flooding React.
const PROGRESS_THROTTLE_MS = 100;

// Transport is the little that the send loop below actually needs from a
// connection: hand it bytes, tell it how much is queued, and wait until the
// queue drains. A WebRTC DataChannel is one; the signaling socket, used as a
// relay when the two browsers cannot reach each other directly, is another.
export type Transport = {
  send(data: ArrayBuffer | string): void;
  bufferedAmount(): number;
  waitDrain(signal: AbortSignal): Promise<void>;
};

export function dataChannelTransport(channel: RTCDataChannel): Transport {
  channel.bufferedAmountLowThreshold = BUFFERED_AMOUNT_LOW_THRESHOLD;
  return {
    send: (data) => (typeof data === "string" ? channel.send(data) : channel.send(data)),
    bufferedAmount: () => channel.bufferedAmount,
    waitDrain: (signal) => waitForBufferedAmountLow(channel, signal),
  };
}

// relayTransport pushes the same framing through the signaling socket, so the
// bytes travel browser -> server -> browser. Slower and it costs the server
// bandwidth, but it needs nothing beyond the HTTPS connection both peers
// already have — which is the whole point: it works from behind the symmetric
// NATs and firewalls that WebRTC hole punching cannot get through.
export function relayTransport(socket: SendSocket, peerId: string): Transport {
  return {
    send: (data) => socket.sendRelay(peerId, data),
    bufferedAmount: () => socket.bufferedAmount,
    waitDrain: (signal) => pollDrain(socket, signal),
  };
}

// A WebSocket has no "bufferedamountlow" event, so draining has to be polled.
// 50ms is far shorter than the time it takes to push 1MB over any link worth
// using, so this never becomes the bottleneck.
function pollDrain(socket: SendSocket, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setInterval(() => {
      if (signal.aborted) {
        clearInterval(timer);
        reject(new Error("transfer aborted"));
        return;
      }
      if (socket.bufferedAmount <= BUFFERED_AMOUNT_LOW_THRESHOLD) {
        clearInterval(timer);
        resolve();
      }
    }, 50);
  });
}

// Streams `file` over an already-open transport: one JSON header message,
// then raw ArrayBuffer chunks, then a JSON {eof:true} sentinel. The receiving
// end (ReceiveAssembler) mirrors this exact framing, whichever transport
// carried it.
export async function sendFileOver(
  transport: Transport,
  file: File,
  onProgress: (sentBytes: number) => void,
  signal: AbortSignal,
): Promise<void> {
  const meta: FileMeta = {
    name: file.name,
    size: file.size,
    mime: file.type || "application/octet-stream",
  };
  transport.send(JSON.stringify({ header: meta }));

  let sent = 0;
  let lastProgressAt = 0;
  let fileOffset = 0;
  while (fileOffset < file.size) {
    if (signal.aborted) throw new Error("transfer aborted");

    // One real file read per block...
    const blockEnd = Math.min(fileOffset + READ_BLOCK_SIZE, file.size);
    const block = await file.slice(fileOffset, blockEnd).arrayBuffer();
    fileOffset = blockEnd;

    // ...then split it into CHUNK_SIZE messages with zero further I/O.
    let blockPos = 0;
    while (blockPos < block.byteLength) {
      if (signal.aborted) throw new Error("transfer aborted");
      if (transport.bufferedAmount() > BUFFERED_AMOUNT_LOW_THRESHOLD) {
        await transport.waitDrain(signal);
      }
      const chunkEnd = Math.min(blockPos + CHUNK_SIZE, block.byteLength);
      transport.send(block.slice(blockPos, chunkEnd));
      sent += chunkEnd - blockPos;
      blockPos = chunkEnd;

      const now = performance.now();
      if (now - lastProgressAt >= PROGRESS_THROTTLE_MS || sent >= file.size) {
        lastProgressAt = now;
        onProgress(sent);
      }
    }
  }
  transport.send(JSON.stringify({ eof: true }));
}

function waitForBufferedAmountLow(channel: RTCDataChannel, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    function onLow() {
      channel.removeEventListener("bufferedamountlow", onLow);
      signal.removeEventListener("abort", onAbort);
      resolve();
    }
    function onAbort() {
      channel.removeEventListener("bufferedamountlow", onLow);
      reject(new Error("transfer aborted"));
    }
    channel.addEventListener("bufferedamountlow", onLow);
    signal.addEventListener("abort", onAbort, { once: true });
  });
}

// MAX_RECEIVE_SIZE is a hard ceiling on a single incoming transfer. The whole
// file is assembled in memory (chunks[] -> Blob) before it's handed to the
// browser, so without a cap a peer could OOM the tab either by declaring a
// gigantic size or by streaming far more bytes than it declared. 2GB is well
// past any realistic LAN drop while still bounding memory.
export const MAX_RECEIVE_SIZE = 2 * 1024 * 1024 * 1024; // 2GB

// ReceiveAssembler turns a sequence of DataChannel messages back into a
// Blob. Feed it every channel.onmessage event in order; it figures out
// header vs. binary chunk vs. EOF from the framing sendFileOverChannel
// produces (JSON strings for control messages, ArrayBuffer for data).
export class ReceiveAssembler {
  private meta: FileMeta | null = null;
  private chunks: ArrayBuffer[] = [];
  private receivedBytes = 0;
  private lastProgressAt = 0;
  private failed = false;

  constructor(
    private onProgress: (receivedBytes: number, totalBytes: number) => void,
    private onComplete: (file: Blob, meta: FileMeta) => void,
    private onError: (reason: string) => void,
  ) {}

  feed(data: ArrayBuffer | string): void {
    if (this.failed) return;

    if (typeof data === "string") {
      const parsed = JSON.parse(data) as { header?: FileMeta; eof?: boolean };
      if (parsed.header) {
        if (!(parsed.header.size >= 0) || parsed.header.size > MAX_RECEIVE_SIZE) {
          this.fail("declared file size is too large to accept");
          return;
        }
        this.meta = parsed.header;
        this.chunks = [];
        this.receivedBytes = 0;
        this.lastProgressAt = 0;
        return;
      }
      if (parsed.eof && this.meta) {
        const blob = new Blob(this.chunks, { type: this.meta.mime });
        this.onComplete(blob, this.meta);
      }
      return;
    }

    this.chunks.push(data);
    this.receivedBytes += data.byteLength;
    if (!this.meta) return;

    // A peer that streams more than it declared is buggy or hostile; stop
    // before the buffered chunks can grow past the declared (already-capped)
    // size and exhaust memory.
    if (this.receivedBytes > this.meta.size) {
      this.fail("sender exceeded the declared file size");
      return;
    }

    const now = performance.now();
    if (now - this.lastProgressAt >= PROGRESS_THROTTLE_MS || this.receivedBytes >= this.meta.size) {
      this.lastProgressAt = now;
      this.onProgress(this.receivedBytes, this.meta.size);
    }
  }

  private fail(reason: string): void {
    this.failed = true;
    this.chunks = []; // release buffered memory immediately
    this.onError(reason);
  }
}
