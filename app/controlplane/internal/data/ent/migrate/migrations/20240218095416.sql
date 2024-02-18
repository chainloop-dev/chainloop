-- Update existing data in "organizations" table
-- Make the content RFC 1123 compliant
UPDATE organizations
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
    FROM organizations
)
UPDATE organizations AS o
SET name = CONCAT(o.name, '-', nn.rn - 1)
FROM numbered_names AS nn
WHERE o.id = nn.id AND nn.rn > 1;

-- Modify "organizations" table
ALTER TABLE "organizations" ALTER COLUMN "name" DROP DEFAULT;

-- Create index "organizations_name_key" to table: "organizations"
CREATE UNIQUE INDEX "organizations_name_key" ON "organizations" ("name");
