-- atlas:txmode none

-- Drop index "workflowrun_created_at_id" from table: "workflow_runs"
DROP INDEX "workflowrun_created_at_id";
-- Drop index "workflowrun_created_at_state" from table: "workflow_runs"
DROP INDEX "workflowrun_created_at_state";
-- Drop index "workflowrun_created_at_workflow_id" from table: "workflow_runs"
DROP INDEX "workflowrun_created_at_workflow_id";
-- Create index "workflowrun_state_created_at" to table: "workflow_runs"
CREATE INDEX CONCURRENTLY "workflowrun_state_created_at" ON "workflow_runs" ("state", "created_at" DESC);
-- Create index "workflowrun_workflow_id_created_at" to table: "workflow_runs"
CREATE INDEX CONCURRENTLY "workflowrun_workflow_id_created_at" ON "workflow_runs" ("workflow_id", "created_at" DESC);
-- Create index "workflowrun_workflow_id_state_created_at" to table: "workflow_runs"
CREATE INDEX CONCURRENTLY "workflowrun_workflow_id_state_created_at" ON "workflow_runs" ("workflow_id", "state", "created_at" DESC);
