-- Update existing data in "workflows" table
-- Make the name and project RFC 1123 compliant
UPDATE workflows
SET name = regexp_replace(
             lower(name), 
             '[^a-z0-9-]', 
             '-', 
             'g'
         );

-- and project
UPDATE workflows
SET project = regexp_replace(
             lower(name), 
             '[^a-z0-9-]', 
             '-', 
             'g'
         );

-- Append suffixes to duplicates
WITH numbered_names AS (
    SELECT 
        id,
        name,
        ROW_NUMBER() OVER (PARTITION BY name ORDER BY id) AS rn
    FROM workflows
)
UPDATE workflows AS o
SET name = CONCAT(o.name, '-', nn.rn - 1)
FROM numbered_names AS nn
WHERE o.id = nn.id AND nn.rn > 1;


WITH numbered_projects AS (
    SELECT 
        id,
        project,
        ROW_NUMBER() OVER (PARTITION BY name ORDER BY id) AS rn
    FROM workflows
)
UPDATE workflows AS o
SET name = CONCAT(o.project, '-', np.rn - 1)
FROM numbered_projects AS np
WHERE o.id = np.id AND np.rn > 1;

-- Create index "workflow_name_organization_id" to table: "workflows"
CREATE UNIQUE INDEX "workflow_name_organization_id" ON "workflows" ("name", "organization_id");
