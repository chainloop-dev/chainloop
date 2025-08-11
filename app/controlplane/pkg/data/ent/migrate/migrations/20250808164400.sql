-- Modify "workflow_contracts" table
ALTER TABLE "workflow_contracts" ADD COLUMN "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;

UPDATE "workflow_contracts" SET "updated_at" = "created_at";

