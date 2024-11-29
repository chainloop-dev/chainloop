-- Drop index "workflowrun_created_at_id" from table: "workflow_runs"
DROP INDEX "workflowrun_created_at_id";
-- Drop index "workflowrun_created_at_state" from table: "workflow_runs"
DROP INDEX "workflowrun_created_at_state";
-- Drop index "workflowrun_created_at_workflow_id" from table: "workflow_runs"
DROP INDEX "workflowrun_created_at_workflow_id";
-- Create index "workflowrun_created_at_id" to table: "workflow_runs"
CREATE INDEX "workflowrun_created_at_id" ON "workflow_runs" ("created_at" DESC, "id");
-- Create index "workflowrun_created_at_state" to table: "workflow_runs"
CREATE INDEX "workflowrun_created_at_state" ON "workflow_runs" ("created_at" DESC, "state");
-- Create index "workflowrun_created_at_workflow_id" to table: "workflow_runs"
CREATE INDEX "workflowrun_created_at_workflow_id" ON "workflow_runs" ("created_at" DESC, "workflow_id");
