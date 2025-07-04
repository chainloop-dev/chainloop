-- Drop index "workflowcontract_name_organization_workflow_contracts" from table: "workflow_contracts"
DROP INDEX "workflowcontract_name_organization_workflow_contracts";
-- Modify "workflow_contracts" table
ALTER TABLE "workflow_contracts" ADD COLUMN "scoped_resource_type" character varying NULL, ADD COLUMN "scoped_resource_id" uuid NULL;
-- Create index "workflowcontract_name_organization_workflow_contracts" to table: "workflow_contracts"
CREATE UNIQUE INDEX "workflowcontract_name_organization_workflow_contracts" ON "workflow_contracts" ("name", "organization_workflow_contracts") WHERE ((deleted_at IS NULL) AND (scoped_resource_type IS NULL));
-- Create index "workflowcontract_name_scoped_resource_type_scoped_resource_id" to table: "workflow_contracts"
CREATE UNIQUE INDEX "workflowcontract_name_scoped_resource_type_scoped_resource_id" ON "workflow_contracts" ("name", "scoped_resource_type", "scoped_resource_id") WHERE (deleted_at IS NULL);
