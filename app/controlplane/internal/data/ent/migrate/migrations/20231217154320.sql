-- Modify "referrers" table
ALTER TABLE "referrers" ADD COLUMN "metadata" jsonb NULL, ADD COLUMN "annotations" jsonb NULL;
