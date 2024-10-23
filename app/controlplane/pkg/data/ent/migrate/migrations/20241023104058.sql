UPDATE "workflows" SET project_id = projects.id FROM "projects" 
  WHERE workflows.project_old = projects.name AND workflows.organization_id = projects.organization_id;

-- Cleanup the redundant data
ALTER TABLE "workflows" ALTER COLUMN "project_id" SET NOT NULL;