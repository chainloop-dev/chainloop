-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD COLUMN "is_system" boolean NOT NULL DEFAULT false, ADD COLUMN "workflow_id" uuid NULL;
