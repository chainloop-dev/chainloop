-- atlas:txmode none

-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" ADD COLUMN "policy_status" character varying NULL, ADD COLUMN "policy_evaluations_total" integer NULL, ADD COLUMN "policy_evaluations_passed" integer NULL, ADD COLUMN "policy_evaluations_skipped" integer NULL, ADD COLUMN "policy_violations_count" integer NULL, ADD COLUMN "policy_has_gates" boolean NULL;
-- Create index "workflowrun_policy_status" to table: "workflow_runs"
CREATE INDEX CONCURRENTLY "workflowrun_policy_status" ON "workflow_runs" ("policy_status");
-- Create partial index "workflowrun_policy_has_gates" on rows where gates were in effect
CREATE INDEX CONCURRENTLY "workflowrun_policy_has_gates" ON "workflow_runs" ("policy_has_gates") WHERE (policy_has_gates = true);
