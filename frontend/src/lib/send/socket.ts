import type { PeerInfo, ServerMessage, SignalPayload } from "./protocol";

// Width of the ASCII peer-ID prefix on a relay frame. Must match
// sendhub.IDLen in the backend — both ends slice at exactly this offset.
const PEER_ID_LEN = 32;

// First byte of a relay payload. The hub forwards the payload untouched, so
// the two browsers need their own marker for "JSON control frame" vs "file
// bytes": a DataChannel gets that distinction for free from string vs
// ArrayBuffer, a binary-only relay does not.
const RELAY_BINARY = 0;
const RELAY_TEXT = 1;

// SendSocket is a thin event-callback wrapper around the raw WebSocket to
// the backend's anonymous signaling hub. It knows the wire format
// (ServerMessage) but nothing about WebRTC — that's the caller's job.
export class SendSocket {
  private ws: WebSocket;
  onHello?: (selfId: string, selfName: string) => void;
  onRoster?: (peers: PeerInfo[]) => void;
  onSignal?: (fromId: string, fromName: string, payload: SignalPayload) => void;
  onSignalError?: (message: string) => void;
  // Relayed file data from a peer whose direct connection never came up.
  onRelay?: (fromId: string, data: ArrayBuffer | string) => void;
  onClose?: () => void;

  constructor(url: string) {
    this.ws = new WebSocket(url);
    this.ws.binaryType = "arraybuffer";
    this.ws.onmessage = (event) => {
      if (event.data instanceof ArrayBuffer) {
        this.handleRelay(event.data);
        return;
      }
      let msg: ServerMessage;
      try {
        msg = JSON.parse(event.data as string) as ServerMessage;
      } catch {
        return;
      }
      switch (msg.type) {
        case "hello":
          this.onHello?.(msg.selfId, msg.selfName);
          break;
        case "roster":
          this.onRoster?.(msg.peers);
          break;
        case "signal":
          this.onSignal?.(msg.from, msg.fromName, msg.payload);
          break;
        case "error":
          this.onSignalError?.(msg.error);
          break;
      }
    };
    this.ws.onclose = () => this.onClose?.();
  }

  private handleRelay(frame: ArrayBuffer) {
    if (frame.byteLength <= PEER_ID_LEN) return;
    const fromId = new TextDecoder().decode(frame.slice(0, PEER_ID_LEN));
    const kind = new Uint8Array(frame, PEER_ID_LEN, 1)[0];
    const body = frame.slice(PEER_ID_LEN + 1);
    this.onRelay?.(fromId, kind === RELAY_TEXT ? new TextDecoder().decode(body) : body);
  }

  sendSignal(to: string, payload: SignalPayload) {
    this.ws.send(JSON.stringify({ type: "signal", to, payload }));
  }

  // sendRelay pushes one frame of an in-progress transfer through the hub:
  // [peer id][kind byte][body]. Only used when WebRTC could not connect the
  // two browsers directly.
  sendRelay(to: string, data: ArrayBuffer | string) {
    const body = typeof data === "string" ? new TextEncoder().encode(data) : new Uint8Array(data);
    const frame = new Uint8Array(PEER_ID_LEN + 1 + body.byteLength);
    frame.set(new TextEncoder().encode(to), 0);
    frame[PEER_ID_LEN] = typeof data === "string" ? RELAY_TEXT : RELAY_BINARY;
    frame.set(body, PEER_ID_LEN + 1);
    this.ws.send(frame);
  }

  get bufferedAmount() {
    return this.ws.bufferedAmount;
  }

  close() {
    this.ws.close();
  }
}
