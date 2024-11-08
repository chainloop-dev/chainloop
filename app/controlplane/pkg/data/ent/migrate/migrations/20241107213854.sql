-- we mark the latest version of each project as prerelease and unset the rest
WITH ranked_versions AS (
  SELECT id,
         ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY created_at DESC) as rn
  FROM project_versions
)
UPDATE project_versions
SET prerelease = (id IN (
  SELECT id
  FROM ranked_versions
  WHERE rn = 1
));
