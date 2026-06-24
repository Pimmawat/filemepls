ALTER TABLE share_links DROP CONSTRAINT chk_share_target;
ALTER TABLE share_links ALTER COLUMN file_id SET NOT NULL;
ALTER TABLE share_links DROP COLUMN folder_id;
ALTER TABLE share_links DROP COLUMN target_type;

ALTER TABLE files DROP COLUMN parent_id;

DROP TABLE folders;
