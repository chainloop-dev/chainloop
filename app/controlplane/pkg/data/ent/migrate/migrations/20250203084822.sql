-- Create "attestations" table
CREATE TABLE "attestations" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, "bundle" bytea NOT NULL, "workflowrun_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "attestations_workflow_runs_attestation_bundle" FOREIGN KEY ("workflowrun_id") REFERENCES "workflow_runs" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "attestations_workflowrun_id_key" to table: "attestations"
CREATE UNIQUE INDEX "attestations_workflowrun_id_key" ON "attestations" ("workflowrun_id");
