-- In a previous migration we created projects for already deleted workflows
-- This code makes sure to soft-delete these invalid projects
-- Update projects.deleted_at where all associated workflows are deleted
UPDATE projects p
SET deleted_at = (
    SELECT MIN(w.deleted_at)
    FROM workflows w
    WHERE w.project_id = p.id
)
WHERE EXISTS (
    SELECT 1
    FROM workflows w
    WHERE w.project_id = p.id
    GROUP BY w.project_id
    HAVING COUNT(*) = COUNT(w.deleted_at)
)
AND p.deleted_at IS NULL;
