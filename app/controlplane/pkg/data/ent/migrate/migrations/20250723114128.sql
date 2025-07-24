-- Modify "memberships" table
-- NOTE: by default, foreign keys are not generated for performance reasons in cases where we do a soft-deletion.
--       In this particular case, enabling cascade deletion makes sense to automatically remove inherited rows
ALTER TABLE "memberships" ADD COLUMN "parent_id" uuid NULL, ADD CONSTRAINT "memberships_memberships_children" FOREIGN KEY ("parent_id") REFERENCES "memberships" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
