-- Create index "workflowrun_state_finished_at" to table: "workflow_runs"
CREATE INDEX "workflowrun_state_finished_at" ON "workflow_runs" ("state", "finished_at");
