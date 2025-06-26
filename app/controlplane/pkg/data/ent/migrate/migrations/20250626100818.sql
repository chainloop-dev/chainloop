-- Drop index "apitoken_name_organization_id" from table: "api_tokens"
DROP INDEX "apitoken_name_organization_id";
-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD COLUMN "project_id" uuid NULL;
-- Create index "apitoken_name_organization_id" to table: "api_tokens"
CREATE UNIQUE INDEX "apitoken_name_organization_id" ON "api_tokens" ("name", "organization_id") WHERE ((revoked_at IS NULL) AND (project_id IS NULL));
-- Create index "apitoken_name_project_id" to table: "api_tokens"
CREATE UNIQUE INDEX "apitoken_name_project_id" ON "api_tokens" ("name", "project_id") WHERE ((revoked_at IS NULL) AND (project_id IS NOT NULL));
