-- Record the scope of every token from before the scope columns existed: the one a new token of
-- the same shape records. Its project, else its organization, else the instance: before those
-- columns, a token with no organization was always an instance token.
-- Safe to rerun: it repairs a row written without a scope by a control plane from before the
-- scope columns (a rolling upgrade or a rollback), and leaves every recorded scope alone.
UPDATE "api_tokens"
SET "scope"    = CASE
                   WHEN "project_id" IS NOT NULL THEN 'project'
                   WHEN "organization_id" IS NOT NULL THEN 'organization'
                   ELSE 'instance'
                 END,
    "scope_id" = COALESCE("project_id", "organization_id")
WHERE "scope" IS NULL;
