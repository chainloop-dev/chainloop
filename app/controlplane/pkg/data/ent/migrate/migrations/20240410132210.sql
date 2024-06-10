-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD COLUMN "name" character varying NULL;
-- update existing values
UPDATE "api_tokens" SET "name" = split_part("id"::text, '-', 1) WHERE "name" IS NULL;
-- Create index "apitoken_name_organization_id" to table: "api_tokens"
CREATE UNIQUE INDEX "apitoken_name_organization_id" ON "api_tokens" ("name", "organization_id") WHERE (revoked_at IS NULL);
-- re-enable constraint
ALTER TABLE "api_tokens" ALTER COLUMN "name" SET NOT NULL;