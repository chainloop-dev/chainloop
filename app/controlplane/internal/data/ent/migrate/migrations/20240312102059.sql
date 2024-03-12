-- Update existing data in "workflow_contracts" table
-- Make the name RFC 1123 compliant
UPDATE workflow_contracts
SET name = regexp_replace(
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
    FROM workflow_contracts
)

UPDATE workflow_contracts AS o
SET name = CONCAT(o.name, '-', nn.rn - 1)
FROM numbered_names AS nn
WHERE o.id = nn.id AND nn.rn > 1;

-- Create index "workflowcontract_name_organization_workflow_contracts" to table: "workflow_contracts"
CREATE UNIQUE INDEX "workflowcontract_name_organization_workflow_contracts" ON "workflow_contracts" ("name", "organization_workflow_contracts");
