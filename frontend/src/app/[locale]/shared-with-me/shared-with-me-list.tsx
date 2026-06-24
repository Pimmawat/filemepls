"use client";

import { useState } from "react";
import { useFormatter, useTranslations } from "next-intl";
import { Folder } from "lucide-react";

import { useRouter } from "@/i18n/navigation";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { type SharedWithMe } from "@/lib/api";
import { FileDetailsDialog } from "../files/file-details-dialog";

function formatSize(bytes: number): string {
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

export function SharedWithMeList({ initial }: { initial: SharedWithMe }) {
  const t = useTranslations("SharedWithMe");
  const tFiles = useTranslations("Files");
  const format = useFormatter();
  const router = useRouter();
  const [detailsFileId, setDetailsFileId] = useState<string | null>(null);

  const isEmpty = initial.folders.length === 0 && initial.files.length === 0;

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl">{t("title")}</h1>

      {isEmpty ? (
        <p className="text-muted-foreground">{t("empty")}</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{tFiles("colName")}</TableHead>
              <TableHead>{tFiles("colSize")}</TableHead>
              <TableHead>{tFiles("colType")}</TableHead>
              <TableHead>{tFiles("colCreated")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {initial.folders.map((folder) => (
              <TableRow
                key={folder.id}
                className="cursor-pointer"
                onClick={() => router.push(`/files/${folder.id}`)}
              >
                <TableCell className="max-w-xs truncate font-medium">
                  <Folder className="mr-1.5 -mt-0.5 inline-block size-4 text-muted-foreground" />{" "}
                  {folder.name}
                </TableCell>
                <TableCell>—</TableCell>
                <TableCell>—</TableCell>
                <TableCell>
                  {format.dateTime(new Date(folder.createdAt), {
                    dateStyle: "short",
                    timeStyle: "medium",
                  })}
                </TableCell>
              </TableRow>
            ))}
            {initial.files.map((file) => (
              <TableRow
                key={file.id}
                className="cursor-pointer"
                onClick={() => setDetailsFileId(file.id)}
              >
                <TableCell className="max-w-xs truncate font-medium" title={file.name}>
                  {file.name || (
                    <span className="font-mono text-xs text-muted-foreground">
                      {file.hash.slice(0, 12)}
                    </span>
                  )}
                </TableCell>
                <TableCell>{formatSize(file.size)}</TableCell>
                <TableCell>{file.mime}</TableCell>
                <TableCell>
                  {format.dateTime(new Date(file.createdAt), {
                    dateStyle: "short",
                    timeStyle: "medium",
                  })}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <FileDetailsDialog
        fileId={detailsFileId}
        open={detailsFileId !== null}
        onOpenChange={(open) => {
          if (!open) setDetailsFileId(null);
        }}
      />
    </div>
  );
}
