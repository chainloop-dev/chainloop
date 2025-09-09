-- atlas:txmode none

-- Drop index "workflowrun_state_created_at" from table: "workflow_runs"
DROP INDEX "workflowrun_state_created_at";
-- Create index "workflowrun_state_created_at" to table: "workflow_runs"
CREATE INDEX CONCURRENTLY "workflowrun_state_created_at" ON "workflow_runs" ("state", "created_at");
