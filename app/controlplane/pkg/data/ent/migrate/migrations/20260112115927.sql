-- Make organization_id nullable to support instance-level API tokens
ALTER TABLE "api_tokens" ALTER COLUMN "organization_id" DROP NOT NULL;

-- Create index "apitoken_name" to table: "api_tokens"
CREATE UNIQUE INDEX "apitoken_name" ON "api_tokens" ("name") WHERE ((revoked_at IS NULL) AND (organization_id IS NULL));

