-- Modify "cas_mappings" table
ALTER TABLE "cas_mappings" DROP CONSTRAINT "cas_mappings_workflow_runs_workflow_run", ALTER COLUMN "workflow_run_id" SET NOT NULL;
-- Create index "casmapping_organization_id" to table: "cas_mappings"
CREATE INDEX "casmapping_organization_id" ON "cas_mappings" ("organization_id");
-- Create index "casmapping_workflow_run_id" to table: "cas_mappings"
CREATE INDEX "casmapping_workflow_run_id" ON "cas_mappings" ("workflow_run_id");
-- Rename an index from "workflowrun_created_at_workflow_workflowruns" to "workflowrun_created_at_workflow_id"
ALTER INDEX "workflowrun_created_at_workflow_workflowruns" RENAME TO "workflowrun_created_at_workflow_id";
-- Rename an index from "workflowrun_workflow_workflowruns" to "workflowrun_workflow_id"
ALTER INDEX "workflowrun_workflow_workflowruns" RENAME TO "workflowrun_workflow_id";
