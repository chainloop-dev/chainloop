-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "api_token_inactivity_threshold_days" bigint NULL;
