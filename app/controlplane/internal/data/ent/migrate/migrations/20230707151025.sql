-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" ADD COLUMN "cas_backend_refs" jsonb NULL;
