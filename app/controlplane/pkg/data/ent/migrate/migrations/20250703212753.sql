-- Drop index "workflowcontract_name_organization_workflow_contracts" from table: "workflow_contracts"
DROP INDEX "workflowcontract_name_organization_workflow_contracts";
-- Modify "workflow_contracts" table
ALTER TABLE "workflow_contracts" ADD COLUMN "project_id" uuid NULL;
-- Create index "workflowcontract_name_organization_workflow_contracts" to table: "workflow_contracts"
CREATE UNIQUE INDEX "workflowcontract_name_organization_workflow_contracts" ON "workflow_contracts" ("name", "organization_workflow_contracts") WHERE ((deleted_at IS NULL) AND (project_id IS NULL));
-- Create index "workflowcontract_name_project_id" to table: "workflow_contracts"
CREATE UNIQUE INDEX "workflowcontract_name_project_id" ON "workflow_contracts" ("name", "project_id") WHERE ((deleted_at IS NULL) AND (project_id IS NOT NULL));
