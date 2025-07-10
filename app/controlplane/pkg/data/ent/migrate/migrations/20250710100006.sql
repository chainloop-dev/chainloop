-- Modify "memberships" table
ALTER TABLE "memberships" RENAME COLUMN "user_memberships" TO "user_id";
ALTER TABLE "memberships" RENAME COLUMN "organization_memberships" TO "organization_id";
