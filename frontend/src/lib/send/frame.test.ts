// Round-trips the relay framing that lib/send/socket.ts writes and reads.
// Run with `npm run test:send`. The frame layout is shared with the Go hub
// (sendhub.RelayBinary): if PEER_ID_LEN here and IDLen there ever disagree,
// every relayed chunk is misrouted or decoded as garbage, silently — no
// exception, no error message, just a transfer that never completes.
import test from "node:test";
import assert from "node:assert/strict";

import { SendSocket } from "./socket";

class FakeWS {
  binaryType = "";
  sent: unknown[] = [];
  onmessage: ((e: { data: unknown }) => void) | null = null;
  onclose: (() => void) | null = null;
  bufferedAmount = 0;
  send(data: unknown) {
    this.sent.push(data);
  }
  close() {}
}

function makeSocket(): { socket: SendSocket; fake: FakeWS } {
  const fake = new FakeWS();
  (globalThis as unknown as Record<string, unknown>).WebSocket = function () {
    return fake;
  };
  return { socket: new SendSocket("ws://test"), fake };
}

const PEER = "a".repeat(32);

test("a binary frame round-trips with the peer ID intact", () => {
  const { socket, fake } = makeSocket();
  socket.sendRelay(PEER, new TextEncoder().encode("file bytes").buffer as ArrayBuffer);

  const frame = fake.sent[0] as Uint8Array;
  assert.equal(new TextDecoder().decode(frame.slice(0, 32)), PEER, "ID prefix");
  assert.equal(frame.byteLength, 32 + 1 + "file bytes".length, "ID + kind byte + payload");

  // The hub swaps the destination prefix for the sender's before forwarding;
  // both are the same width, so echoing the frame back is a faithful loopback.
  const got: Array<{ from: string; data: ArrayBuffer | string }> = [];
  socket.onRelay = (from, data) => got.push({ from, data });
  fake.onmessage!({ data: frame.buffer });

  assert.equal(got.length, 1);
  assert.equal(got[0].from, PEER);
  assert.ok(got[0].data instanceof ArrayBuffer, "file bytes must stay bytes");
  assert.equal(new TextDecoder().decode(got[0].data as ArrayBuffer), "file bytes");
});

test("a control frame arrives as a string, not as bytes", () => {
  const { socket, fake } = makeSocket();
  socket.sendRelay(PEER, JSON.stringify({ header: { name: "x", size: 1, mime: "text/plain" } }));

  const got: Array<ArrayBuffer | string> = [];
  socket.onRelay = (_from, data) => got.push(data);
  fake.onmessage!({ data: (fake.sent[0] as Uint8Array).buffer });

  // ReceiveAssembler splits control frames from data purely on this type, so a
  // header that arrives as an ArrayBuffer would be appended to the file.
  assert.equal(typeof got[0], "string");
  assert.equal(JSON.parse(got[0] as string).header.name, "x");
});

test("a frame too short to hold an ID is ignored, not decoded", () => {
  const { socket, fake } = makeSocket();
  let called = false;
  socket.onRelay = () => {
    called = true;
  };
  fake.onmessage!({ data: new Uint8Array(10).buffer });
  assert.equal(called, false);
});
