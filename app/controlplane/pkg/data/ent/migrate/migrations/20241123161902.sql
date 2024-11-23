ALTER TABLE "cas_mappings" RENAME COLUMN "cas_mapping_workflow_run" TO "workflow_run_id";
-- Rename a column from "cas_mapping_organization" to "organization_id"
ALTER TABLE "cas_mappings" RENAME COLUMN "cas_mapping_organization" TO "organization_id";

ALTER TABLE "workflow_runs" RENAME COLUMN "workflow_workflowruns" TO "workflow_id";
ALTER TABLE "workflow_runs" ALTER COLUMN "workflow_id" SET NOT NULL;
