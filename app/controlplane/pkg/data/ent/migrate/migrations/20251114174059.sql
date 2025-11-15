-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" ADD COLUMN "has_policy_violations" boolean NULL;
