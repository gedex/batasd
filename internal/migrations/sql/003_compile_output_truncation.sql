ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS compile_output_truncated boolean NOT NULL DEFAULT false;
