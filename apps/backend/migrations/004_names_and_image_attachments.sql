ALTER TABLE users
    ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';

ALTER TABLE project_attachments
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'text';

ALTER TABLE project_attachments
    DROP CONSTRAINT IF EXISTS project_attachments_kind_check;

ALTER TABLE project_attachments
    ADD CONSTRAINT project_attachments_kind_check
    CHECK (kind IN ('text', 'image'));
