-- Drop index "membership_organization_memberships_user_memberships" from table: "memberships"
DROP INDEX "membership_organization_memberships_user_memberships";
-- Modify "memberships" table
ALTER TABLE "memberships" ALTER COLUMN "organization_memberships" DROP NOT NULL, ALTER COLUMN "user_memberships" DROP NOT NULL;
-- Create index "membership_organization_memberships_user_memberships" to table: "memberships"
CREATE INDEX "membership_organization_memberships_user_memberships" ON "memberships" ("organization_memberships", "user_memberships");
