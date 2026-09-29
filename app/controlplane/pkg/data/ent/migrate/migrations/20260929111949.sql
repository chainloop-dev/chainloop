-- Transactional on purpose: a concurrent rebuild of a unique index leaves a window with no
-- uniqueness on the organization-level token name, and cannot be retried if interrupted.

-- Drop index "apitoken_name_organization_id" from table: "api_tokens"
DROP INDEX "apitoken_name_organization_id";
-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD CONSTRAINT "apitoken_scope_id_presence" CHECK ((scope_id IS NOT NULL) = ((scope IS NOT NULL) AND ((scope)::text <> 'instance'::text))), ADD CONSTRAINT "apitoken_scope_matches_token" CHECK ((scope IS NULL) OR (((scope)::text = 'organization'::text) AND (project_id IS NULL) AND (NOT (scope_id IS DISTINCT FROM organization_id))) OR (((scope)::text = 'project'::text) AND (NOT (scope_id IS DISTINCT FROM project_id))) OR (((scope)::text = 'instance'::text) AND (organization_id IS NULL) AND (project_id IS NULL)) OR (((scope)::text = 'product'::text) AND (organization_id IS NOT NULL) AND (project_id IS NULL))), ADD COLUMN "scope" character varying NULL, ADD COLUMN "scope_id" uuid NULL;
-- Create index "apitoken_name_organization_id" to table: "api_tokens"
CREATE UNIQUE INDEX "apitoken_name_organization_id" ON "api_tokens" ("name", "organization_id") WHERE ((revoked_at IS NULL) AND (project_id IS NULL) AND ((scope IS NULL) OR ((scope)::text <> 'product'::text)));
-- Create index "apitoken_name_scope_id" to table: "api_tokens"
CREATE UNIQUE INDEX "apitoken_name_scope_id" ON "api_tokens" ("name", "scope_id") WHERE ((revoked_at IS NULL) AND ((scope)::text = 'product'::text));
