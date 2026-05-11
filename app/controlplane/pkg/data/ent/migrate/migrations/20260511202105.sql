-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" ADD COLUMN "policy_violations_suppressed" integer NULL;
