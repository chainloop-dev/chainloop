-- Create "project_versions" table
CREATE TABLE "project_versions" ("id" uuid NOT NULL, "version" character varying NOT NULL, "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, "deleted_at" timestamptz NULL, "project_id" uuid NOT NULL, PRIMARY KEY ("id"));
-- Create index "projectversion_version_project_id" to table: "project_versions"
CREATE UNIQUE INDEX "projectversion_version_project_id" ON "project_versions" ("version", "project_id") WHERE (deleted_at IS NULL);
