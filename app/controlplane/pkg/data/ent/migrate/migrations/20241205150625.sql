-- Modify "project_versions" table
ALTER TABLE "project_versions" ADD COLUMN "workflow_run_count" bigint NOT NULL DEFAULT 0;

-- Update the "project_versions" table by adding the proper count of workflow runs for each project version
WITH workflow_run_counts AS (
  SELECT
    "version_id" AS id,
    COUNT(*) AS run_count
  FROM
    "workflow_runs"
  GROUP BY
    "version_id"
)
UPDATE "project_versions"
SET "workflow_run_count" = workflow_run_counts.run_count
FROM workflow_run_counts
WHERE workflow_run_counts.id = "project_versions"."id";

