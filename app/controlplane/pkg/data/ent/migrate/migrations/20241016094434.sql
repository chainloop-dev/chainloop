-- Create index "workflow_organization_id_id" to table: "workflows"
CREATE UNIQUE INDEX "workflow_organization_id_id" ON "workflows" ("organization_id", "id") WHERE (deleted_at IS NULL);
