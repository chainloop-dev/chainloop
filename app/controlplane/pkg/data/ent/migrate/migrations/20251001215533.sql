-- Modify "project_versions" table
ALTER TABLE "project_versions" ADD COLUMN "last_run_at" timestamptz NULL;
