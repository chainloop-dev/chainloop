-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "block_on_policy_failure" boolean NOT NULL DEFAULT false;
