-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD COLUMN "project_ids" jsonb NULL;
