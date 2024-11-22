-- Create index "workflow_workflow_contract" to table: "workflows"
CREATE INDEX "workflow_workflow_contract" ON "workflows" ("workflow_contract") WHERE (deleted_at IS NULL);
