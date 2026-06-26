import type { FileMeta } from "./protocol";

export const CHUNK_SIZE = 16 * 1024;
// Pause sending once this many bytes are buffered but not yet handed to
// the network, and resume on the channel's "bufferedamountlow" event —
// without this, a fast sender on a slow/congested DataChannel would queue
// the entire file in memory before any of it actually goes out.
const BUFFERED_AMOUNT_LOW_THRESHOLD = 1 << 20; // 1MB

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

  let offset = 0;
  while (offset < file.size) {
    if (signal.aborted) throw new Error("transfer aborted");
    if (channel.bufferedAmount > BUFFERED_AMOUNT_LOW_THRESHOLD) {
      await waitForBufferedAmountLow(channel, signal);
    }
    const slice = file.slice(offset, offset + CHUNK_SIZE);
    const buf = await slice.arrayBuffer();
    channel.send(buf);
    offset += buf.byteLength;
    onProgress(offset);
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
    if (this.meta) this.onProgress(this.receivedBytes, this.meta.size);
  }
}
