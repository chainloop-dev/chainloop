-- Drop index "workflow_name_project_organization_id" from table: "workflows"
DROP INDEX "workflow_name_project_organization_id";
-- Modify "workflows" table
ALTER TABLE "workflows" ADD COLUMN "project_id" uuid, ADD CONSTRAINT "workflows_projects_workflows" FOREIGN KEY ("project_id") REFERENCES "projects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "workflow_name_organization_id_project_id" to table: "workflows"
CREATE UNIQUE INDEX "workflow_name_organization_id_project_id" ON "workflows" ("name", "organization_id", "project_id") WHERE (deleted_at IS NULL);
-- Rename a column from "project" to "project_old"
ALTER TABLE "workflows" RENAME COLUMN "project" TO "project_old";

-- update existing data
UPDATE "workflows" SET project_id = projects.id FROM "projects" 
  WHERE workflows.project_old = projects.name AND workflows.organization_id = projects.organization_id;
ALTER TABLE "workflows" ALTER COLUMN "project_id" SET NOT NULL;

ALTER TABLE "workflows" ALTER COLUMN "project_old" DROP NOT NULL;