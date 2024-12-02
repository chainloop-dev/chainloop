-- Create index "workflow_organization_id" to table: "workflows"
CREATE INDEX "workflow_organization_id" ON "workflows" ("organization_id") WHERE (deleted_at IS NULL);
