-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" ADD COLUMN "attestation_state" bytea NULL;
