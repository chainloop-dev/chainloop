-- Modify "project_versions" table
ALTER TABLE "project_versions" ADD COLUMN "latest" boolean NOT NULL DEFAULT false;

-- Reset all latest flags to false
UPDATE "project_versions" SET "latest" = false;

-- Set latest to true for the most recent version of each project
WITH latest_versions AS (
  SELECT DISTINCT ON (project_id) id
  FROM "project_versions"
  WHERE deleted_at IS NULL
  ORDER BY project_id, created_at DESC
)
UPDATE "project_versions" pv
SET "latest" = true
FROM latest_versions lv
WHERE pv.id = lv.id;


