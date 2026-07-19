CREATE INDEX IF NOT EXISTS idx_submissions_created_token ON submissions (created_at DESC, token DESC);
CREATE INDEX IF NOT EXISTS idx_submissions_language_slug ON submissions (language_slug);
