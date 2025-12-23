-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "disable_requirements_auto_matching" boolean NOT NULL DEFAULT false;
