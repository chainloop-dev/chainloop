-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" ADD COLUMN "sandbox_identity" jsonb NULL;
