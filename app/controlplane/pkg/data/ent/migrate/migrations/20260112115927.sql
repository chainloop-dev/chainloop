-- Make organization_id nullable to support instance-level API tokens
ALTER TABLE "api_tokens" ALTER COLUMN "organization_id" DROP NOT NULL;
