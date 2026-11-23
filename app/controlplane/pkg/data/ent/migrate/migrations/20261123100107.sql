-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD COLUMN "allowed_digests" jsonb NULL;
