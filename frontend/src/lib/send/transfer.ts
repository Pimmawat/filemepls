import type { FileMeta } from "./protocol";

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

// Streams `file` over an already-open RTCDataChannel: one JSON header
// message, then raw ArrayBuffer chunks, then a JSON {eof:true} sentinel.
// The receiving end (ReceiveAssembler) mirrors this exact framing.
export async function sendFileOverChannel(
  channel: RTCDataChannel,
  file: File,
  onProgress: (sentBytes: number) => void,
  signal: AbortSignal,
): Promise<void> {
  channel.bufferedAmountLowThreshold = BUFFERED_AMOUNT_LOW_THRESHOLD;
  const meta: FileMeta = {
    name: file.name,
    size: file.size,
    mime: file.type || "application/octet-stream",
  };
  channel.send(JSON.stringify({ header: meta }));

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
      if (channel.bufferedAmount > BUFFERED_AMOUNT_LOW_THRESHOLD) {
        await waitForBufferedAmountLow(channel, signal);
      }
      const chunkEnd = Math.min(blockPos + CHUNK_SIZE, block.byteLength);
      channel.send(block.slice(blockPos, chunkEnd));
      sent += chunkEnd - blockPos;
      blockPos = chunkEnd;

      const now = performance.now();
      if (now - lastProgressAt >= PROGRESS_THROTTLE_MS || sent >= file.size) {
        lastProgressAt = now;
        onProgress(sent);
      }
    }
  }
  channel.send(JSON.stringify({ eof: true }));
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

// ReceiveAssembler turns a sequence of DataChannel messages back into a
// Blob. Feed it every channel.onmessage event in order; it figures out
// header vs. binary chunk vs. EOF from the framing sendFileOverChannel
// produces (JSON strings for control messages, ArrayBuffer for data).
export class ReceiveAssembler {
  private meta: FileMeta | null = null;
  private chunks: ArrayBuffer[] = [];
  private receivedBytes = 0;
  private lastProgressAt = 0;

  constructor(
    private onProgress: (receivedBytes: number, totalBytes: number) => void,
    private onComplete: (file: Blob, meta: FileMeta) => void,
  ) {}

  feed(data: ArrayBuffer | string): void {
    if (typeof data === "string") {
      const parsed = JSON.parse(data) as { header?: FileMeta; eof?: boolean };
      if (parsed.header) {
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

    const now = performance.now();
    if (now - this.lastProgressAt >= PROGRESS_THROTTLE_MS || this.receivedBytes >= this.meta.size) {
      this.lastProgressAt = now;
      this.onProgress(this.receivedBytes, this.meta.size);
    }
  }
}
