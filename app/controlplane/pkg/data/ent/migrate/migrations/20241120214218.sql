-- atlas:txmode none

DROP INDEX IF EXISTS "workflowrun_created_at_id";
CREATE INDEX CONCURRENTLY "workflowrun_created_at_id" ON "workflow_runs" ("created_at", "id");

DROP INDEX IF EXISTS "workflowrun_created_at_state";
CREATE INDEX CONCURRENTLY "workflowrun_created_at_state" ON "workflow_runs" ("created_at", "state");

DROP INDEX IF EXISTS "workflowrun_created_at_workflow_workflowruns";
CREATE INDEX CONCURRENTLY "workflowrun_created_at_workflow_workflowruns" ON "workflow_runs" ("created_at", "workflow_workflowruns");

DROP INDEX IF EXISTS "workflowrun_workflow_workflowruns";
CREATE INDEX CONCURRENTLY "workflowrun_workflow_workflowruns" ON "workflow_runs" ("workflow_workflowruns");
