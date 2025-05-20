-- atlas:txmode none

-- Create index "workflowrun_created_at" to table: "workflow_runs"
CREATE INDEX CONCURRENTLY "workflowrun_created_at" ON "workflow_runs" ("created_at" DESC);
