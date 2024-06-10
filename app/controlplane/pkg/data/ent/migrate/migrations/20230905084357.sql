-- Modify "workflows" table
ALTER TABLE "workflows" ADD COLUMN "public" boolean NOT NULL DEFAULT false;
