-- Create index "workflowrun_created_at_workflow_workflowruns" to table: "workflow_runs"
CREATE INDEX "workflowrun_created_at_workflow_workflowruns" ON "workflow_runs" ("created_at", "workflow_workflowruns");
