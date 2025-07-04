-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD COLUMN "last_used_at" timestamptz NULL;
