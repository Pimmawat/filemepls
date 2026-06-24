CREATE TABLE access_grants (
    id          UUID PRIMARY KEY,
    target_type TEXT NOT NULL,
    file_id     UUID REFERENCES files(id) ON DELETE CASCADE,
    folder_id   UUID REFERENCES folders(id) ON DELETE CASCADE,
    grantee_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_by  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE access_grants ADD CONSTRAINT chk_access_grant_target CHECK (
    (target_type = 'file'   AND file_id IS NOT NULL AND folder_id IS NULL) OR
    (target_type = 'folder' AND folder_id IS NOT NULL AND file_id IS NULL)
);

CREATE UNIQUE INDEX idx_access_grants_file_grantee ON access_grants (file_id, grantee_id) WHERE file_id IS NOT NULL;
CREATE UNIQUE INDEX idx_access_grants_folder_grantee ON access_grants (folder_id, grantee_id) WHERE folder_id IS NOT NULL;
CREATE INDEX idx_access_grants_grantee_id ON access_grants (grantee_id);
