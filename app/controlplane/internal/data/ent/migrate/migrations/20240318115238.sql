-- Modify "robot_accounts" table
ALTER TABLE "robot_accounts" ADD CONSTRAINT "robot_accounts_organizations_organization" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
