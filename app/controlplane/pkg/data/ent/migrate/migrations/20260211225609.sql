-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "api_token_inactivity_threshold_days" bigint NULL DEFAULT 30;
-- Backfill existing organizations with the default threshold
UPDATE "organizations" SET "api_token_inactivity_threshold_days" = 30 WHERE "api_token_inactivity_threshold_days" IS NULL;
