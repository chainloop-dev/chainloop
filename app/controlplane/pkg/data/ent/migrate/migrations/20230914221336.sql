-- Create index "workflowrun_attestation_digest" to table: "workflow_runs"
CREATE INDEX "workflowrun_attestation_digest" ON "workflow_runs" ("attestation_digest");
