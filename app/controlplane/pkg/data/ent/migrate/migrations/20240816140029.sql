-- Modify "workflow_contract_versions" table
ALTER TABLE "workflow_contract_versions" ADD COLUMN "raw_body_format" character varying NULL;

-- Set empty values but not null, this is so we can set the not null constraint later
UPDATE "workflow_contract_versions" SET "raw_body_format" = '' WHERE "raw_body_format" IS NULL;

ALTER TABLE "workflow_contract_versions" ALTER COLUMN "raw_body_format" SET NOT NULL;