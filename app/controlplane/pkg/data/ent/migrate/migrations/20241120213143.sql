-- atlas:txmode none

DROP INDEX workflowrun_attestation_digest;
CREATE INDEX CONCURRENTLY "workflowrun_attestation_digest" ON "workflow_runs" ("attestation_digest");