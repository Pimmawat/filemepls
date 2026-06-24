CREATE TABLE users (
    id           UUID PRIMARY KEY,
    email        TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    provider     TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE blobs (
    hash       CHAR(64) PRIMARY KEY,
    size       BIGINT NOT NULL CHECK (size > 0),
    mime       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE files (
    id         UUID PRIMARY KEY,
    hash       CHAR(64) NOT NULL REFERENCES blobs(hash),
    size       BIGINT NOT NULL CHECK (size > 0),
    mime       TEXT NOT NULL,
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_files_hash ON files (hash);
CREATE INDEX idx_files_owner_id ON files (owner_id);

CREATE TABLE share_links (
    id             UUID PRIMARY KEY,
    token          TEXT NOT NULL,
    file_id        UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    expires_at     TIMESTAMPTZ,
    password_hash  TEXT,
    max_downloads  INTEGER,
    download_count INTEGER NOT NULL DEFAULT 0,
    visibility     TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_share_links_token ON share_links (token);
CREATE INDEX idx_share_links_file_id ON share_links (file_id);
