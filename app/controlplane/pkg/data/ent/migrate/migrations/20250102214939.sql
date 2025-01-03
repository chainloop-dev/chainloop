-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "block_on_policy_violation" boolean NOT NULL DEFAULT false;
