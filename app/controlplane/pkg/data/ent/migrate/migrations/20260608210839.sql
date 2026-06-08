-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "block_attestations_on_released_versions" boolean NOT NULL DEFAULT false;
