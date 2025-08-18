-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "deleted_at" timestamptz NULL;
-- Create index "organization_name" to table: "organizations"
CREATE UNIQUE INDEX "organization_name" ON "organizations" ("name") WHERE (deleted_at IS NULL);
