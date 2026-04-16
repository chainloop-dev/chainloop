-- atlas:txmode none

-- Step 1: Rename any existing user-created "v0" versions to "v0.0"
-- (avoids conflict when the empty-string default is renamed to "v0")
UPDATE project_versions
SET version = 'v0.0'
WHERE version = 'v0'
    AND deleted_at IS NULL;

-- Step 2: Rename all default "" versions to "v0"
UPDATE project_versions
SET version = 'v0'
WHERE version = ''
    AND deleted_at IS NULL;

-- Step 3: Change column default from '' to 'v0'
ALTER TABLE "project_versions" ALTER COLUMN "version" SET DEFAULT 'v0';
