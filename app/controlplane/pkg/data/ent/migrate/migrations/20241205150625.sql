-- Modify "project_versions" table
ALTER TABLE "project_versions" ADD COLUMN "workflow_run_count" bigint NOT NULL DEFAULT 0;

-- Update the "project_versions" table by adding the proper count of workflow runs for each project version
UPDATE "project_versions" SET "workflow_run_count" = (
  SELECT COUNT(*)
  FROM "workflow_runs"
  WHERE "workflow_runs"."version_id" = "project_versions"."id"
);
