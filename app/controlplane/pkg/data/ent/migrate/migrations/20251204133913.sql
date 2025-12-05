-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "prevent_project_scoped_contracts" boolean NOT NULL DEFAULT false;
