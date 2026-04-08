-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "suspended" boolean NOT NULL DEFAULT false;
