-- Modify "workflow_contracts" table
ALTER TABLE "workflow_contracts" ADD COLUMN "scoped_resource_type" character varying NULL, ADD COLUMN "scoped_resource_id" uuid NULL;
