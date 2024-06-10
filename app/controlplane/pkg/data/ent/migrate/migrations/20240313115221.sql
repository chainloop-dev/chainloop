-- Drop index "workflowcontract_name_organization_workflow_contracts" from table: "workflow_contracts"
DROP INDEX "workflowcontract_name_organization_workflow_contracts";
-- Create index "workflowcontract_name_organization_workflow_contracts" to table: "workflow_contracts"
CREATE UNIQUE INDEX "workflowcontract_name_organization_workflow_contracts" ON "workflow_contracts" ("name", "organization_workflow_contracts") WHERE (deleted_at IS NULL);
-- Drop index "workflow_name_organization_id" from table: "workflows"
DROP INDEX "workflow_name_organization_id";
-- Create index "workflow_name_organization_id" to table: "workflows"
CREATE UNIQUE INDEX "workflow_name_organization_id" ON "workflows" ("name", "organization_id") WHERE (deleted_at IS NULL);
