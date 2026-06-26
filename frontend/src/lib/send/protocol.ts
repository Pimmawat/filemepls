// Shared wire-protocol types for the anonymous LAN-send feature. The
// backend's sendhub package only relays these envelopes; everything under
// `payload` for a "signal" message is opaque to it and meaningful only to
// the two browsers exchanging it (WebRTC offer/answer/ICE candidates plus
// the file metadata sent alongside an offer).

export type PeerInfo = { id: string; name: string };

export type FileMeta = { name: string; size: number; mime: string };

export type SignalPayload =
  | { kind: "offer"; sdp: string; file: FileMeta }
  | { kind: "answer"; sdp: string }
  | { kind: "ice"; candidate: RTCIceCandidateInit }
  | { kind: "reject" };

export type ServerMessage =
  | { type: "hello"; selfId: string; selfName: string }
  | { type: "roster"; peers: PeerInfo[] }
  | { type: "signal"; from: string; fromName: string; payload: SignalPayload }
  | { type: "error"; error: string };
