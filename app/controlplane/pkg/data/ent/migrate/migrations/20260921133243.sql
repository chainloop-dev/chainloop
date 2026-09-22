-- Transactional on purpose: a concurrent rebuild of a unique index leaves a window with no
-- uniqueness on the organization-level token name, and cannot be retried if interrupted.

-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD COLUMN "scope" character varying NULL, ADD COLUMN "scope_id" uuid NULL, ADD COLUMN "scope_name" character varying NULL;
-- Drop index "apitoken_name_organization_id" from table: "api_tokens"
DROP INDEX "apitoken_name_organization_id";
-- Create index "apitoken_name_organization_id" to table: "api_tokens", excluding scope-confined tokens
CREATE UNIQUE INDEX "apitoken_name_organization_id" ON "api_tokens" ("name", "organization_id") WHERE ((revoked_at IS NULL) AND (project_id IS NULL) AND (scope_id IS NULL));
-- Create index "apitoken_name_scope_id" to table: "api_tokens"
CREATE UNIQUE INDEX "apitoken_name_scope_id" ON "api_tokens" ("name", "scope_id") WHERE ((revoked_at IS NULL) AND (scope_id IS NOT NULL));
