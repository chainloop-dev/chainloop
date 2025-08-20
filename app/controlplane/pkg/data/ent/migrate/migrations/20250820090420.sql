-- Create default version for projects that don't have any versions
INSERT INTO "project_versions" ("id", "project_id", "version", "prerelease", "latest", "created_at", "updated_at")
SELECT 
    gen_random_uuid() as id,
    p.id as project_id,
    '' as version,
    true as prerelease,
    true as latest,
    p.created_at as created_at,
    p.updated_at as updated_at
FROM "projects" p
WHERE p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM "project_versions" pv 
    WHERE pv.project_id = p.id 
      AND pv.deleted_at IS NULL
  );