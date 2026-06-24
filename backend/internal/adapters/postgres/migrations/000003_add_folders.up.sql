CREATE TABLE folders (
    id         UUID PRIMARY KEY,
    name       TEXT NOT NULL,
    parent_id  UUID REFERENCES folders(id),
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_folders_owner_id ON folders (owner_id);
CREATE INDEX idx_folders_parent_id ON folders (parent_id);

ALTER TABLE files ADD COLUMN parent_id UUID REFERENCES folders(id);
CREATE INDEX idx_files_parent_id ON files (parent_id);

ALTER TABLE share_links
    ADD COLUMN target_type TEXT NOT NULL DEFAULT 'file',
    ADD COLUMN folder_id   UUID REFERENCES folders(id) ON DELETE CASCADE,
    ALTER COLUMN file_id DROP NOT NULL;

ALTER TABLE share_links ADD CONSTRAINT chk_share_target CHECK (
    (target_type = 'file'   AND file_id IS NOT NULL AND folder_id IS NULL) OR
    (target_type = 'folder' AND folder_id IS NOT NULL AND file_id IS NULL)
);
CREATE INDEX idx_share_links_folder_id ON share_links (folder_id);
