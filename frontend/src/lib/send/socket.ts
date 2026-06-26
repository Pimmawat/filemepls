import type { PeerInfo, ServerMessage, SignalPayload } from "./protocol";

// SendSocket is a thin event-callback wrapper around the raw WebSocket to
// the backend's anonymous signaling hub. It knows the wire format
// (ServerMessage) but nothing about WebRTC — that's the caller's job.
export class SendSocket {
  private ws: WebSocket;
  onHello?: (selfId: string, selfName: string) => void;
  onRoster?: (peers: PeerInfo[]) => void;
  onSignal?: (fromId: string, fromName: string, payload: SignalPayload) => void;
  onSignalError?: (message: string) => void;
  onClose?: () => void;

  constructor(url: string) {
    this.ws = new WebSocket(url);
    this.ws.onmessage = (event) => {
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

  sendSignal(to: string, payload: SignalPayload) {
    this.ws.send(JSON.stringify({ type: "signal", to, payload }));
  }

  close() {
    this.ws.close();
  }
}
