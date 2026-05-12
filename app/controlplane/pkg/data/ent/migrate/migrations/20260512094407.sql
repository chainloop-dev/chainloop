-- Modify "workflow_contracts" table
ALTER TABLE "workflow_contracts" ADD COLUMN "managed" boolean NOT NULL DEFAULT false;
