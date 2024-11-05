-- Modify "project_versions" table
ALTER TABLE "project_versions" DROP CONSTRAINT "project_versions_projects_versions", ADD CONSTRAINT "project_versions_projects_versions" FOREIGN KEY ("project_id") REFERENCES "projects" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
