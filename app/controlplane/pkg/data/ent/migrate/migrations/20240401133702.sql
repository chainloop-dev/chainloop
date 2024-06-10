-- Modify "integrations" table
ALTER TABLE "integrations" ADD COLUMN "name" character varying NULL;
-- Add names to existing integrations based on the kind and first fragment of the id
UPDATE "integrations" SET "name" = CONCAT(LOWER("kind"), '-', split_part("id"::text, '-', 1)) WHERE "name" IS NULL;
-- Create index "integration_name_organization_integrations" to table: "integrations"
CREATE UNIQUE INDEX "integration_name_organization_integrations" ON "integrations" ("name", "organization_integrations") WHERE (deleted_at IS NULL);
-- Enable the not null constraint
ALTER TABLE "integrations" ALTER COLUMN "name" SET NOT NULL;
