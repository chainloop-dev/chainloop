-- Modify "project_versions" table
ALTER TABLE "project_versions" ALTER COLUMN "version" SET DEFAULT '';
-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" ADD COLUMN "version_id" uuid NULL, ADD CONSTRAINT "workflow_runs_project_versions_runs" FOREIGN KEY ("version_id") REFERENCES "project_versions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
