-- Transactional on purpose: a concurrent rebuild of a unique index leaves a window with no
-- uniqueness on the organization-level token name, and cannot be retried if interrupted.

-- Drop index "apitoken_name_organization_id" from table: "api_tokens"
DROP INDEX "apitoken_name_organization_id";
-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD COLUMN "scope" character varying NULL, ADD COLUMN "scope_id" uuid NULL;
-- Create index "apitoken_name_organization_id" to table: "api_tokens"
CREATE UNIQUE INDEX "apitoken_name_organization_id" ON "api_tokens" ("name", "organization_id") WHERE ((revoked_at IS NULL) AND (project_id IS NULL) AND ((scope IS NULL) OR ((scope)::text <> 'product'::text)));
-- Create index "apitoken_name_scope_id" to table: "api_tokens"
CREATE UNIQUE INDEX "apitoken_name_scope_id" ON "api_tokens" ("name", "scope_id") WHERE ((revoked_at IS NULL) AND ((scope)::text = 'product'::text));
