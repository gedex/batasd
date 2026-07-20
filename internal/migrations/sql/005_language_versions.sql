ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS language_version text NOT NULL DEFAULT '';

UPDATE submissions
SET language_slug = CASE language_slug
        WHEN 'c-gcc' THEN 'c'
        WHEN 'cpp-gcc' THEN 'cpp'
        WHEN 'go-1.24' THEN 'go'
        WHEN 'java-21' THEN 'java'
        WHEN 'node-22' THEN 'node'
        WHEN 'php-8.3' THEN 'php'
        WHEN 'python-3.12' THEN 'python'
        WHEN 'rust-1.88' THEN 'rust'
        ELSE language_slug
    END,
    language_version = CASE language_slug
        WHEN 'c' THEN 'gcc'
        WHEN 'c-gcc' THEN 'gcc'
        WHEN 'cpp' THEN 'gcc'
        WHEN 'cpp-gcc' THEN 'gcc'
        WHEN 'go' THEN '1.24'
        WHEN 'go-1.24' THEN '1.24'
        WHEN 'java' THEN '21'
        WHEN 'java-21' THEN '21'
        WHEN 'node' THEN '22'
        WHEN 'node-22' THEN '22'
        WHEN 'php' THEN '8.3'
        WHEN 'php-8.3' THEN '8.3'
        WHEN 'python' THEN '3.12'
        WHEN 'python-3.12' THEN '3.12'
        WHEN 'rust' THEN '1.88'
        WHEN 'rust-1.88' THEN '1.88'
        ELSE language_version
    END
WHERE language_version = '';

ALTER TABLE submissions
    ALTER COLUMN language_version DROP DEFAULT;

CREATE INDEX IF NOT EXISTS idx_submissions_language_version ON submissions (language_version);
