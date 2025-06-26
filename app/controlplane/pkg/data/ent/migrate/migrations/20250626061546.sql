-- Create "groups" table
CREATE TABLE "groups" ("id" uuid NOT NULL, "name" character varying NOT NULL, "description" character varying NULL, "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, "deleted_at" timestamptz NULL, "organization_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "groups_organizations_groups" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "group_name_organization_id" to table: "groups"
CREATE UNIQUE INDEX "group_name_organization_id" ON "groups" ("name", "organization_id") WHERE (deleted_at IS NULL);
-- Create "group_memberships" table
CREATE TABLE "group_memberships" ("id" uuid NOT NULL, "maintainer" boolean NOT NULL DEFAULT false, "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, "deleted_at" timestamptz NULL, "group_id" uuid NOT NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "group_memberships_groups_group" FOREIGN KEY ("group_id") REFERENCES "groups" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "group_memberships_users_user" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "groupmembership_group_id_user_id" to table: "group_memberships"
CREATE UNIQUE INDEX "groupmembership_group_id_user_id" ON "group_memberships" ("group_id", "user_id");
