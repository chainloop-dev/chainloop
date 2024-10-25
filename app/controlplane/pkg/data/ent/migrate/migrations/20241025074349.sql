-- Modify "project_versions" table
ALTER TABLE "project_versions" ADD CONSTRAINT "project_versions_projects_versions" FOREIGN KEY ("project_id") REFERENCES "projects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;

-- Create initial versions for existing projects
INSERT INTO "project_versions" ("id", "project_id", "version", "created_at")
SELECT 
    gen_random_uuid(),
    "id",
    '',
    CURRENT_TIMESTAMP
FROM "projects";
