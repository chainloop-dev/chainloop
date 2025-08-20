-- Fix projects that don't have a latest version set
-- Find projects that have project_versions but no latest version marked
WITH projects_without_latest AS (
    SELECT DISTINCT pv.project_id
    FROM project_versions pv
    WHERE pv.deleted_at IS NULL
    AND pv.project_id NOT IN (
        SELECT DISTINCT project_id 
        FROM project_versions 
        WHERE latest = true 
        AND deleted_at IS NULL
    )
),
-- Get the most recent version for each project that doesn't have a latest
latest_for_missing AS (
    SELECT 
        pv.id,
        pv.project_id,
        ROW_NUMBER() OVER (
            PARTITION BY pv.project_id 
            ORDER BY pv.created_at DESC, pv.id DESC
        ) as rn
    FROM project_versions pv
    INNER JOIN projects_without_latest pwl ON pv.project_id = pwl.project_id
    WHERE pv.deleted_at IS NULL
)
-- Update only the most recent version for projects missing a latest version
UPDATE project_versions 
SET latest = true 
WHERE id IN (
    SELECT id 
    FROM latest_for_missing 
    WHERE rn = 1
);