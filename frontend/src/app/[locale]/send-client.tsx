"use client";

import { useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Laptop2, Send as SendIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";
import { SEND_WS_URL } from "@/lib/api";
import { useSendHub, type IncomingOffer, type OutgoingTransfer } from "./use-send-hub";

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

export function SendClient() {
  const t = useTranslations("Send");
  const { self, peers, incomingQueue, outgoing, incoming, sendFile, acceptIncoming, rejectIncoming } =
    useSendHub(SEND_WS_URL);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const [targetPeer, setTargetPeer] = useState<{ id: string; name: string } | null>(null);

  function pickFileFor(peerId: string, peerName: string) {
    setTargetPeer({ id: peerId, name: peerName });
    fileInputRef.current?.click();
  }

  function handleFileChosen(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file || !targetPeer) return;
    sendFile(targetPeer, file);
    setTargetPeer(null);
  }

  const currentOffer: IncomingOffer | undefined = incomingQueue[0];

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col gap-6 px-4 py-10">
      <div className="space-y-1 text-center">
        <h1 className="text-2xl">{t("title")}</h1>
        <p className="text-muted-foreground">{t("subtitle")}</p>
        {self && (
          <p className="text-sm text-muted-foreground">
            {t("youAre")} <span className="font-medium text-foreground">{self.name}</span>
          </p>
        )}
      </div>

      <input ref={fileInputRef} type="file" className="hidden" onChange={handleFileChosen} />

      <div className="flex flex-col gap-2">
        <h2 className="text-sm font-medium text-muted-foreground">{t("nearbyDevices")}</h2>
        {peers.length === 0 ? (
          <p className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
            {t("noDevices")}
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {peers.map((peer) => (
              <li
                key={peer.id}
                className="flex items-center justify-between gap-3 rounded-lg border p-3"
              >
                <div className="flex items-center gap-2">
                  <Laptop2 className="size-5 text-muted-foreground" />
                  <span>{peer.name}</span>
                </div>
                <Button size="sm" onClick={() => pickFileFor(peer.id, peer.name)}>
                  <SendIcon className="size-4" />
                  {t("sendButton")}
                </Button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {outgoing.length > 0 && (
        <div className="flex flex-col gap-2">
          <h2 className="text-sm font-medium text-muted-foreground">{t("outgoingTitle")}</h2>
          {outgoing.map((transfer) => (
            <OutgoingTransferRow key={transfer.peerId} transfer={transfer} t={t} />
          ))}
        </div>
      )}

      {incoming && (
        <div className="flex flex-col gap-2">
          <h2 className="text-sm font-medium text-muted-foreground">{t("incomingTitle")}</h2>
          <div className="rounded-lg border p-3">
            <p className="mb-2 text-sm">
              {incoming.file.name} {t("fromPeer")} {incoming.fromName}
            </p>
            <Progress value={(incoming.receivedBytes / incoming.file.size) * 100} />
            <div className="mt-1 flex items-center justify-between text-xs text-muted-foreground">
              <span>{incoming.status === "done" ? t("statusDone") : t("statusReceiving")}</span>
              <span>{formatBytes(incoming.receivedBytes)} / {formatBytes(incoming.file.size)}</span>
            </div>
          </div>
        </div>
      )}

      <Dialog open={!!currentOffer} onOpenChange={(open) => !open && currentOffer && rejectIncoming(currentOffer)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("incomingRequestTitle")}</DialogTitle>
          </DialogHeader>
          {currentOffer && (
            <p className="text-sm">
              {t("incomingRequestBody", {
                name: currentOffer.fromName,
                file: currentOffer.file.name,
                size: formatBytes(currentOffer.file.size),
              })}
            </p>
          )}
          <DialogFooter>
            <Button variant="ghost" onClick={() => currentOffer && rejectIncoming(currentOffer)}>
              {t("reject")}
            </Button>
            <Button onClick={() => currentOffer && acceptIncoming(currentOffer)}>{t("accept")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function OutgoingTransferRow({
  transfer,
  t,
}: {
  transfer: OutgoingTransfer;
  t: ReturnType<typeof useTranslations>;
}) {
  const statusLabel =
    transfer.status === "connecting"
      ? t("statusConnecting")
      : transfer.status === "sending"
        ? t("statusSending")
        : transfer.status === "done"
          ? t("statusDone")
          : transfer.status === "rejected"
            ? t("statusRejected")
            : t("statusFailed");

  return (
    <div className="rounded-lg border p-3">
      <p className="mb-2 text-sm">
        {transfer.fileName} {t("toPeer")} {transfer.peerName}
      </p>
      <Progress value={(transfer.sentBytes / transfer.fileSize) * 100} />
      <div className="mt-1 flex items-center justify-between text-xs text-muted-foreground">
        <span>{statusLabel}</span>
        <span>{formatBytes(transfer.sentBytes)} / {formatBytes(transfer.fileSize)}</span>
      </div>
    </div>
  );
}
