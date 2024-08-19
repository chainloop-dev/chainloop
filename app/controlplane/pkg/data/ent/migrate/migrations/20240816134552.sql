-- Modify "workflow_contract_versions" table
ALTER TABLE "workflow_contract_versions" ADD COLUMN "raw_body" bytea NULL;

-- Set empty values but not null, this is so we can set the not null constraint later
UPDATE "workflow_contract_versions" SET "raw_body" = '\x' WHERE "raw_body" IS NULL;

-- Enable the not null constraint
ALTER TABLE "workflow_contract_versions" ALTER COLUMN "raw_body" SET NOT NULL;