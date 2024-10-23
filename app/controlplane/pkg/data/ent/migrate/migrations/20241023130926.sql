-- Modify "workflows" table
ALTER TABLE "workflows" DROP CONSTRAINT "workflows_projects_project", ADD CONSTRAINT "workflows_projects_workflows" FOREIGN KEY ("project_id") REFERENCES "projects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
