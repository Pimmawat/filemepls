"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { SendSocket } from "@/lib/send/socket";
import { ReceiveAssembler, sendFileOverChannel } from "@/lib/send/transfer";
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
      }
    };

    const connections = peerConnections.current;
    const candidates = pendingCandidates.current;
    const controllers = abortControllers.current;
    return () => {
      socket.close();
      connections.forEach((pc) => pc.close());
      connections.clear();
      candidates.clear();
      controllers.forEach((c) => c.abort());
      controllers.clear();
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
      pc.onconnectionstatechange = () => {
        if (pc.connectionState === "failed" || pc.connectionState === "disconnected") {
          patchOutgoing(peer.id, { status: "failed" });
        }
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

      channel.onopen = () => {
        patchOutgoing(peer.id, { status: "sending" });
        sendFileOverChannel(
          channel,
          file,
          (sentBytes) => patchOutgoing(peer.id, { sentBytes }),
          controller.signal,
        )
          .then(() => patchOutgoing(peer.id, { status: "done" }))
          .catch(() => patchOutgoing(peer.id, { status: "failed" }))
          .finally(() => closePeer(peer.id));
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
          closePeer(offer.fromId);
        },
      );

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
