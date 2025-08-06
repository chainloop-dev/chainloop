-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "policies_allowed_domains" jsonb NULL;
