-- Modify "attestations" table
ALTER TABLE "attestations" DROP CONSTRAINT "attestations_workflow_runs_attestation_bundle", ADD CONSTRAINT "attestations_workflow_runs_attestation_bundle" FOREIGN KEY ("workflowrun_id") REFERENCES "workflow_runs" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
