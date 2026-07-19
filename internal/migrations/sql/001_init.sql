CREATE TABLE IF NOT EXISTS submissions (
    token text PRIMARY KEY,
    language_slug text NOT NULL,
    source text NOT NULL,
    input text NOT NULL DEFAULT '',
    expected_output text,
    arguments jsonb NOT NULL DEFAULT '[]'::jsonb,
    compiler_options jsonb NOT NULL DEFAULT '[]'::jsonb,
    limits jsonb NOT NULL DEFAULT '{}'::jsonb,
    additional_files jsonb,
    callback_url text,
    status_code text NOT NULL,
    stdout text,
    stderr text,
    stdout_truncated boolean NOT NULL DEFAULT false,
    stderr_truncated boolean NOT NULL DEFAULT false,
    compile_output text,
    compile_output_truncated boolean NOT NULL DEFAULT false,
    message text,
    exit_code integer,
    exit_signal text,
    time_ms bigint,
    wall_time_ms bigint,
    memory_kb bigint,
    created_at timestamptz NOT NULL,
    queued_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    updated_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_submissions_status_code ON submissions (status_code);
CREATE INDEX IF NOT EXISTS idx_submissions_created_at ON submissions (created_at);

CREATE TABLE IF NOT EXISTS callback_attempts (
    id bigserial PRIMARY KEY,
    submission_token text NOT NULL REFERENCES submissions(token) ON DELETE CASCADE,
    attempt integer NOT NULL,
    status_code integer,
    error text,
    created_at timestamptz NOT NULL DEFAULT now()
);
