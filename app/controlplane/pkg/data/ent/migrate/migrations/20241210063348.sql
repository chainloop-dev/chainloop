-- Create index "workflowrun_version_id_workflow_id" to table: "workflow_runs"
CREATE INDEX CONCURRENTLY "workflowrun_version_id_workflow_id" ON "workflow_runs" ("version_id", "workflow_id");
