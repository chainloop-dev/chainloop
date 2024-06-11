-- Modify "cas_backends" table
ALTER TABLE "cas_backends" ADD COLUMN "name" character varying NULL;
-- Add data
UPDATE "cas_backends" SET "name" = CONCAT(LOWER("provider"), '-', split_part("id"::text, '-', 1)) WHERE "name" IS NULL;
-- Create index "casbackend_name_organization_cas_backends" to table: "cas_backends"
CREATE UNIQUE INDEX "casbackend_name_organization_cas_backends" ON "cas_backends" ("name", "organization_cas_backends") WHERE (deleted_at IS NULL);
-- Enable the not null constraint
ALTER TABLE "cas_backends" ALTER COLUMN "name" SET NOT NULL;
