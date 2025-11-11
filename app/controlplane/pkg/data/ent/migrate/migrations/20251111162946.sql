-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "prevent_implicit_workflow_creation" boolean NOT NULL DEFAULT false;
