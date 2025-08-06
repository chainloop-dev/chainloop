-- find project_versions referenced by cas_mapping with a wrong project_id
with WRONG_MAPPINGS as (select id from cas_mappings
                        WHERE workflow_run_id is NULL AND project_id is not NULL
                          AND project_id NOT IN (SELECT id FROM projects))
-- update cas_mapping.project_id to the correct project_id
UPDATE cas_mappings cm
SET project_id = (SELECT project_id FROM project_versions pv
                  WHERE pv.id = cm.project_id)
WHERE id IN (SELECT id FROM WRONG_MAPPINGS);