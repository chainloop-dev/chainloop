-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" ADD COLUMN "contract_revision_used" bigint, ADD COLUMN "contract_revision_latest_available" bigint;

-- Set previous values to 0
UPDATE "workflow_runs" SET "contract_revision_used" = 0, "contract_revision_latest_available" = 0;

-- Force the new columns to be not null
ALTER TABLE "workflow_runs" ALTER COLUMN "contract_revision_used" SET NOT NULL;
ALTER TABLE "workflow_runs" ALTER COLUMN "contract_revision_latest_available" SET NOT NULL;

