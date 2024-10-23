-- Create "projects" table
CREATE TABLE "projects" ("id" uuid NOT NULL, "name" character varying NOT NULL, "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, "deleted_at" timestamptz NULL, "organization_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "projects_organizations_projects" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "project_name_organization_id" to table: "projects"
CREATE UNIQUE INDEX "project_name_organization_id" ON "projects" ("name", "organization_id") WHERE (deleted_at IS NULL);
