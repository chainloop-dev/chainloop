-- Modify "project_versions" table
ALTER TABLE "project_versions" ADD COLUMN "prerelease" boolean NOT NULL DEFAULT true;
