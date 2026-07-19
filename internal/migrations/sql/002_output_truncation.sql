ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS stdout_truncated boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS stderr_truncated boolean NOT NULL DEFAULT false;
