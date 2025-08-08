-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "policies_allowed_hostnames" jsonb NULL;
