-- Create index "workflow_name_project_organization_id" to table: "workflows"
CREATE UNIQUE INDEX "workflow_name_project_organization_id" ON "workflows" ("name", "project", "organization_id") WHERE (deleted_at IS NULL);
