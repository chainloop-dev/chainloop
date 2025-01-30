-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" ADD COLUMN "bundle_id" uuid NULL;
-- Create "bundles" table
CREATE TABLE "bundles" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, "bundle" bytea NOT NULL, "workflowrun_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "bundles_workflow_runs_bundle" FOREIGN KEY ("workflowrun_id") REFERENCES "workflow_runs" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "bundles_workflowrun_id_key" to table: "bundles"
CREATE UNIQUE INDEX "bundles_workflowrun_id_key" ON "bundles" ("workflowrun_id");
