"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { SendSocket } from "@/lib/send/socket";
import {
  ReceiveAssembler,
  dataChannelTransport,
  relayTransport,
  sendFileOver,
  type Transport,
} from "@/lib/send/transfer";
import type { FileMeta, PeerInfo, SignalPayload } from "@/lib/send/protocol";

const ICE_SERVERS: RTCIceServer[] = [{ urls: "stun:stun.l.google.com:19302" }];

export type OutgoingTransfer = {
  peerId: string;
  peerName: string;
  fileName: string;
  fileSize: number;
  sentBytes: number;
  status: "connecting" | "sending" | "done" | "rejected" | "failed";
};

export type IncomingOffer = {
  fromId: string;
  fromName: string;
  file: FileMeta;
  sdp: string;
};

export type IncomingTransfer = {
  fromId: string;
  fromName: string;
  file: FileMeta;
  receivedBytes: number;
  status: "receiving" | "done" | "failed";
  blobUrl?: string;
};

function downloadBlob(url: string, filename: string) {
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
}

// useSendHub owns the WebSocket signaling connection plus every active
// RTCPeerConnection, and exposes plain state + actions to the UI. One
// outgoing transfer per peer at a time (keyed by peer ID); incoming offers
// queue so a second offer arriving while one is pending isn't dropped.
export function useSendHub(wsUrl: string) {
  const [self, setSelf] = useState<PeerInfo | null>(null);
  const [peers, setPeers] = useState<PeerInfo[]>([]);
  const [incomingQueue, setIncomingQueue] = useState<IncomingOffer[]>([]);
  const [outgoing, setOutgoing] = useState<Map<string, OutgoingTransfer>>(new Map());
  const [incoming, setIncoming] = useState<IncomingTransfer | null>(null);

  const socketRef = useRef<SendSocket | null>(null);
  const peerConnections = useRef(new Map<string, RTCPeerConnection>());
  const pendingCandidates = useRef(new Map<string, RTCIceCandidateInit[]>());
  const abortControllers = useRef(new Map<string, AbortController>());
  // Keyed by sender ID so relayed chunks, which arrive on the shared socket
  // rather than on a per-peer data channel, can be routed to the right one.
  const assemblers = useRef(new Map<string, ReceiveAssembler>());

  const patchOutgoing = useCallback((peerId: string, patch: Partial<OutgoingTransfer>) => {
    setOutgoing((prev) => {
      const current = prev.get(peerId);
      if (!current) return prev;
      const next = new Map(prev);
      next.set(peerId, { ...current, ...patch });
      return next;
    });
  }, []);

  const closePeer = useCallback((peerId: string) => {
    peerConnections.current.get(peerId)?.close();
    peerConnections.current.delete(peerId);
    pendingCandidates.current.delete(peerId);
    abortControllers.current.get(peerId)?.abort();
    abortControllers.current.delete(peerId);
    assemblers.current.delete(peerId);
  }, []);

  const flushCandidates = useCallback((peerId: string, pc: RTCPeerConnection) => {
    const queued = pendingCandidates.current.get(peerId);
    if (!queued) return;
    queued.forEach((c) => void pc.addIceCandidate(c).catch(() => {}));
    pendingCandidates.current.delete(peerId);
  }, []);

  useEffect(() => {
    const socket = new SendSocket(wsUrl);
    socketRef.current = socket;

    socket.onHello = (selfId, selfName) => setSelf({ id: selfId, name: selfName });
    socket.onRoster = (list) => setPeers(list);
    socket.onRelay = (fromId, data) => assemblers.current.get(fromId)?.feed(data);

    socket.onSignal = (fromId, fromName, payload: SignalPayload) => {
      switch (payload.kind) {
        case "offer":
          setIncomingQueue((q) => [...q, { fromId, fromName, file: payload.file, sdp: payload.sdp }]);
          break;
        case "answer": {
          const pc = peerConnections.current.get(fromId);
          if (!pc) return;
          void pc
            .setRemoteDescription({ type: "answer", sdp: payload.sdp })
            .then(() => flushCandidates(fromId, pc));
          break;
        }
        case "ice": {
          const pc = peerConnections.current.get(fromId);
          if (!pc || !pc.remoteDescription) {
            const queue = pendingCandidates.current.get(fromId) ?? [];
            queue.push(payload.candidate);
            pendingCandidates.current.set(fromId, queue);
            return;
          }
          void pc.addIceCandidate(payload.candidate).catch(() => {});
          break;
        }
        case "reject":
          patchOutgoing(fromId, { status: "rejected" });
          closePeer(fromId);
          break;
        case "ack":
          patchOutgoing(fromId, { status: "done" });
          closePeer(fromId);
          break;
      }
    };

    const connections = peerConnections.current;
    const candidates = pendingCandidates.current;
    const controllers = abortControllers.current;
    const openAssemblers = assemblers.current;
    return () => {
      socket.close();
      connections.forEach((pc) => pc.close());
      connections.clear();
      candidates.clear();
      controllers.forEach((c) => c.abort());
      controllers.clear();
      openAssemblers.clear();
    };
  }, [wsUrl, closePeer, flushCandidates, patchOutgoing]);

  const sendFile = useCallback(
    (peer: PeerInfo, file: File) => {
      const socket = socketRef.current;
      if (!socket) return;

      const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });
      peerConnections.current.set(peer.id, pc);
      const controller = new AbortController();
      abortControllers.current.set(peer.id, controller);

      pc.onicecandidate = (e) => {
        if (e.candidate) socket.sendSignal(peer.id, { kind: "ice", candidate: e.candidate.toJSON() });
      };
      const channel = pc.createDataChannel("file");
      setOutgoing((prev) => {
        const next = new Map(prev);
        next.set(peer.id, {
          peerId: peer.id,
          peerName: peer.name,
          fileName: file.name,
          fileSize: file.size,
          sentBytes: 0,
          status: "connecting",
        });
        return next;
      });

      // Whichever transport wins the race starts the transfer; the other one
      // is then a no-op. The receiver acks over signaling once it has actually
      // assembled the file (see acceptIncoming), and only then is pc closed —
      // tearing down when the local send loop finishes would drop whatever is
      // still buffered or in flight on a real network, since send() only
      // enqueues. A zero-latency loopback test never surfaces that race.
      //
      // ponytail: no switchover mid-transfer. A connection that dies halfway
      // fails the transfer and the user sends again; resuming from an offset
      // needs the receiver to report what it kept, which is a protocol, not a
      // fallback.
      let started = false;
      const startTransfer = (transport: Transport) => {
        if (started) return;
        started = true;
        patchOutgoing(peer.id, { status: "sending" });
        sendFileOver(
          transport,
          file,
          (sentBytes) => patchOutgoing(peer.id, { sentBytes }),
          controller.signal,
        ).catch(() => {
          patchOutgoing(peer.id, { status: "failed" });
          closePeer(peer.id);
        });
      };

      channel.onopen = () => startTransfer(dataChannelTransport(channel));

      pc.onconnectionstatechange = () => {
        // "failed" only happens after ICE has actually tried, which means the
        // receiver answered — so this cannot fire while an offer is still
        // sitting unanswered. Hole punching lost: push the bytes through the
        // signaling socket instead, which is the one path both peers are
        // guaranteed to have. "disconnected" is deliberately not handled: it is
        // routinely transient, and "failed" follows when it is not.
        if (pc.connectionState === "failed") startTransfer(relayTransport(socket, peer.id));
      };

      void pc
        .createOffer()
        .then((offer) => pc.setLocalDescription(offer))
        .then(() => {
          socket.sendSignal(peer.id, {
            kind: "offer",
            sdp: pc.localDescription!.sdp,
            file: { name: file.name, size: file.size, mime: file.type || "application/octet-stream" },
          });
        });
    },
    [closePeer, patchOutgoing],
  );

  const acceptIncoming = useCallback(
    (offer: IncomingOffer) => {
      const socket = socketRef.current;
      if (!socket) return;
      setIncomingQueue((q) => q.filter((o) => o !== offer));

      const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });
      peerConnections.current.set(offer.fromId, pc);

      pc.onicecandidate = (e) => {
        if (e.candidate) socket.sendSignal(offer.fromId, { kind: "ice", candidate: e.candidate.toJSON() });
      };

      setIncoming({
        fromId: offer.fromId,
        fromName: offer.fromName,
        file: offer.file,
        receivedBytes: 0,
        status: "receiving",
      });

      const assembler = new ReceiveAssembler(
        (receivedBytes) =>
          setIncoming((prev) => (prev && prev.fromId === offer.fromId ? { ...prev, receivedBytes } : prev)),
        (blob) => {
          const url = URL.createObjectURL(blob);
          setIncoming((prev) =>
            prev && prev.fromId === offer.fromId ? { ...prev, status: "done", blobUrl: url } : prev,
          );
          downloadBlob(url, offer.file.name);
          // Tell the sender it is safe to close now — only after the file is
          // actually fully assembled here, not just "all chunks sent". Over
          // signaling, not the data channel, because a relayed transfer has no
          // data channel to answer on. See sendFile's "ack" case.
          socket.sendSignal(offer.fromId, { kind: "ack" });
          closePeer(offer.fromId);
        },
        () => {
          // Size cap tripped (declared too large, or sender overran it): drop
          // the transfer instead of letting buffered chunks exhaust the tab.
          setIncoming((prev) =>
            prev && prev.fromId === offer.fromId ? { ...prev, status: "failed" } : prev,
          );
          closePeer(offer.fromId);
        },
      );

      // Fed from whichever transport the sender ends up using: its data
      // channel, or relayed frames arriving on the shared socket.
      assemblers.current.set(offer.fromId, assembler);
      pc.ondatachannel = (e) => {
        e.channel.onmessage = (msg) => assembler.feed(msg.data);
      };

      void pc
        .setRemoteDescription({ type: "offer", sdp: offer.sdp })
        .then(() => {
          flushCandidates(offer.fromId, pc);
          return pc.createAnswer();
        })
        .then((answer) => pc.setLocalDescription(answer))
        .then(() => socket.sendSignal(offer.fromId, { kind: "answer", sdp: pc.localDescription!.sdp }))
        .catch(() => setIncoming((prev) => (prev && prev.fromId === offer.fromId ? { ...prev, status: "failed" } : prev)));
    },
    [closePeer, flushCandidates],
  );

  const rejectIncoming = useCallback((offer: IncomingOffer) => {
    setIncomingQueue((q) => q.filter((o) => o !== offer));
    socketRef.current?.sendSignal(offer.fromId, { kind: "reject" });
  }, []);

  const dismissIncomingTransfer = useCallback(() => setIncoming(null), []);

  const outgoingList = useMemo(() => Array.from(outgoing.values()), [outgoing]);

  return {
    self,
    peers,
    incomingQueue,
    outgoing: outgoingList,
    incoming,
    sendFile,
    acceptIncoming,
    rejectIncoming,
    dismissIncomingTransfer,
  };
}
