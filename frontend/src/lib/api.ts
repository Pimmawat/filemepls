export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8008";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

// Thrown when an upload is cancelled via the AbortSignal passed to
// uploadFile, so callers can distinguish "user cancelled" from a real
// failure and skip showing an error toast.
export class UploadCancelledError extends Error {
  constructor() {
    super("upload cancelled");
  }
}

type FetchOptions = RequestInit & { cookie?: string };

// apiFetch always sends credentials: "include" so the browser attaches the
// httpOnly session cookie cross-port in dev. Server Components can't rely
// on that (an outgoing server-side fetch doesn't inherit the incoming
// request's cookies), so they pass `cookie` explicitly via lib/session.ts.
async function apiFetch(path: string, init: FetchOptions = {}): Promise<Response> {
  const { cookie, headers, ...rest } = init;
  const finalHeaders = new Headers(headers);
  if (cookie) {
    finalHeaders.set("Cookie", cookie);
  }

  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...rest,
    headers: finalHeaders,
    credentials: "include",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error ?? res.statusText);
  }
  return res;
}

export type FileMeta = {
  id: string;
  hash: string;
  size: number;
  mime: string;
  name: string;
  ownerId: string;
  parentId: string | null;
  createdAt: string;
};

export type FolderMeta = {
  id: string;
  name: string;
  ownerId: string;
  parentId: string | null;
  createdAt: string;
};

export type BrowseResult = {
  folder: FolderMeta | null;
  breadcrumb: FolderMeta[];
  subfolders: FolderMeta[];
  files: FileMeta[];
};

export type User = {
  id: string;
  email: string;
  displayName: string;
  provider: string;
  avatarUrl: string;
  createdAt: string;
};

export type Visibility = "public" | "private" | "unlisted";

export type ShareLink = {
  id: string;
  token: string;
  targetType: "file" | "folder";
  fileId: string | null;
  folderId: string | null;
  expiresAt: string | null;
  maxDownloads: number | null;
  downloadCount: number;
  visibility: Visibility;
  createdAt: string;
};

export type PublicShareState = {
  status: "ok" | "expired" | "limit_reached" | "needs_password" | "auth_required" | "not_found";
  targetType?: "file" | "folder";
  file?: FileMeta;
  folder?: BrowseResult;
};

export type CreateShareLinkInput = {
  visibility: Visibility;
  expiresAt?: string;
  maxDownloads?: number;
  password?: string;
};

export type UserSummary = {
  id: string;
  email: string;
  displayName: string;
  avatarUrl: string;
};

export type AccessGrant = {
  id: string;
  targetType: "file" | "folder";
  fileId: string | null;
  folderId: string | null;
  granteeId: string;
  granteeEmail: string;
  granteeName: string;
  granteeAvatarUrl: string;
  createdAt: string;
};

export type SharedWithMe = {
  files: FileMeta[];
  folders: FolderMeta[];
};

export const api = {
  me: (cookie?: string) =>
    apiFetch("/api/auth/me", { cookie }).then((r) => r.json() as Promise<User>),
  logout: () => apiFetch("/api/auth/logout", { method: "POST" }),
  authorizeUrl: (provider: "github" | "google") =>
    `${API_BASE_URL}/api/auth/${provider}/authorize`,
  register: (email: string, password: string, displayName: string) =>
    apiFetch("/api/auth/register", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password, displayName }),
    }).then((r) => r.json() as Promise<User>),
  login: (email: string, password: string) =>
    apiFetch("/api/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    }).then((r) => r.json() as Promise<User>),

  listFiles: (cookie?: string) =>
    apiFetch("/api/files", { cookie }).then((r) => r.json() as Promise<FileMeta[]>),
  // Uses XMLHttpRequest instead of fetch: fetch has no cross-browser way to
  // observe upload progress, while XHR's `upload.onprogress` does. Streamed
  // straight from disk by the browser either way — no in-memory buffering.
  uploadFile: (
    file: File,
    parentId: string | null,
    onProgress?: (loaded: number, total: number) => void,
    signal?: AbortSignal,
  ) => {
    return new Promise<FileMeta>((resolve, reject) => {
      const form = new FormData();
      form.append("file", file);
      if (parentId) form.append("parentId", parentId);

      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${API_BASE_URL}/api/files`);
      xhr.withCredentials = true;

      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress?.(e.loaded, e.total);
      };

      xhr.onload = () => {
        let body: unknown;
        try {
          body = JSON.parse(xhr.responseText);
        } catch {
          body = null;
        }
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(body as FileMeta);
        } else {
          const message =
            (body as { error?: string } | null)?.error ?? xhr.statusText ?? "upload failed";
          reject(new ApiError(xhr.status, message));
        }
      };
      xhr.onerror = () => reject(new ApiError(0, "network error during upload"));
      xhr.onabort = () => reject(new UploadCancelledError());

      if (signal?.aborted) {
        reject(new UploadCancelledError());
        return;
      }
      signal?.addEventListener("abort", () => xhr.abort());

      xhr.send(form);
    });
  },
  deleteFile: (id: string) => apiFetch(`/api/files/${id}`, { method: "DELETE" }),
  // Exposed as a plain URL (used in <a href>), not fetched into JS, so the
  // browser handles Range requests/streaming/resume natively.
  downloadUrl: (id: string) => `${API_BASE_URL}/api/files/${id}/download`,
  getFile: (id: string, cookie?: string) =>
    apiFetch(`/api/files/${id}`, { cookie }).then((r) => r.json() as Promise<FileMeta>),
  moveFile: (id: string, parentId: string | null) =>
    apiFetch(`/api/files/${id}/move`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ parentId }),
    }),

  // Folders. browse(folderId) lists a folder's contents (or root, if
  // folderId is omitted): its breadcrumb, subfolders, and files — a single
  // ID resolves the full breadcrumb server-side, no client-side recursion.
  browse: (folderId?: string, cookie?: string) =>
    apiFetch(`/api/folders/browse${folderId ? `?id=${folderId}` : ""}`, { cookie }).then(
      (r) => r.json() as Promise<BrowseResult>,
    ),
  createFolder: (name: string, parentId: string | null) =>
    apiFetch("/api/folders", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, parentId }),
    }).then((r) => r.json() as Promise<FolderMeta>),
  deleteFolder: (id: string) => apiFetch(`/api/folders/${id}`, { method: "DELETE" }),
  moveFolder: (id: string, parentId: string | null) =>
    apiFetch(`/api/folders/${id}/move`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ parentId }),
    }),
  // Exposed as a plain URL, same native-streaming rule as file downloads.
  folderDownloadZipUrl: (id: string) => `${API_BASE_URL}/api/folders/${id}/download`,

  createShareLink: (fileId: string, input: CreateShareLinkInput) =>
    apiFetch(`/api/files/${fileId}/shares`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }).then((r) => r.json() as Promise<ShareLink>),
  // Lists every share link previously created for a file, so one created
  // in an earlier session can be found again and revoked later.
  listShareLinks: (fileId: string) =>
    apiFetch(`/api/files/${fileId}/shares`).then((r) => r.json() as Promise<ShareLink[]>),
  createFolderShareLink: (folderId: string, input: CreateShareLinkInput) =>
    apiFetch(`/api/folders/${folderId}/shares`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }).then((r) => r.json() as Promise<ShareLink>),
  listFolderShareLinks: (folderId: string) =>
    apiFetch(`/api/folders/${folderId}/shares`).then((r) => r.json() as Promise<ShareLink[]>),
  revokeShareLink: (id: string) => apiFetch(`/api/shares/${id}`, { method: "DELETE" }),

  getPublicShare: (token: string, cookie?: string) =>
    apiFetch(`/api/share/${token}`, { cookie }).then(
      (r) => r.json() as Promise<PublicShareState>,
    ),
  // Browses a subfolder within a shared folder (folderId omitted = the
  // share's own root). POST (not GET+query) so a required password never
  // appears in a URL/log/history. Re-verified server-side on every call —
  // there's no session concept for anonymous share access.
  browsePublicFolderShare: (token: string, folderId?: string, password?: string) =>
    apiFetch(`/api/share/${token}/browse`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ folderId, password }),
    }).then((r) => r.json() as Promise<BrowseResult>),
  // Checks a password with no download side effects, so the frontend can
  // pre-flight-check it via fetch before ever submitting/navigating to the
  // real download form — a wrong password then surfaces as a normal in-app
  // error instead of the browser navigating to a raw JSON error response.
  verifySharePassword: (token: string, password: string) =>
    apiFetch(`/api/share/${token}/verify-password`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    }),
  // Exposed as a plain URL for a real <form method="POST" action="..."> so
  // a password-protected download streams as a native browser download,
  // never buffered through JS.
  shareDownloadUrl: (token: string) => `${API_BASE_URL}/api/share/${token}/download`,
  shareZipUrl: (token: string) => `${API_BASE_URL}/api/share/${token}/zip`,
  shareFolderFileDownloadUrl: (token: string, fileId: string) =>
    `${API_BASE_URL}/api/share/${token}/files/${fileId}/download`,

  // Searches users by email substring for the "assign permission" picker.
  // An empty query is never sent (the caller debounces/guards this).
  searchUsers: (query: string) =>
    apiFetch(`/api/users/search?q=${encodeURIComponent(query)}`).then(
      (r) => r.json() as Promise<UserSummary[]>,
    ),
  grantFileAccess: (fileId: string, email: string) =>
    apiFetch(`/api/files/${fileId}/permissions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    }).then((r) => r.json() as Promise<AccessGrant>),
  listFileGrants: (fileId: string) =>
    apiFetch(`/api/files/${fileId}/permissions`).then((r) => r.json() as Promise<AccessGrant[]>),
  grantFolderAccess: (folderId: string, email: string) =>
    apiFetch(`/api/folders/${folderId}/permissions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    }).then((r) => r.json() as Promise<AccessGrant>),
  listFolderGrants: (folderId: string) =>
    apiFetch(`/api/folders/${folderId}/permissions`).then(
      (r) => r.json() as Promise<AccessGrant[]>,
    ),
  revokeGrant: (id: string) => apiFetch(`/api/permissions/${id}`, { method: "DELETE" }),
  sharedWithMe: (cookie?: string) =>
    apiFetch("/api/shared-with-me", { cookie }).then((r) => r.json() as Promise<SharedWithMe>),
};
