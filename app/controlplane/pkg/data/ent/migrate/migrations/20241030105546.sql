-- Create index "project_organization_id" to table: "projects"
CREATE INDEX "project_organization_id" ON "projects" ("organization_id") WHERE (deleted_at IS NULL);
