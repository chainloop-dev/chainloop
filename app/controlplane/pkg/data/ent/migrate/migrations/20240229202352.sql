-- Modify "memberships" table
ALTER TABLE "memberships" ADD COLUMN "role" character varying;

-- Set the existing memberships to the role "role:admin" to not to break compatibility
UPDATE "memberships" SET "role" = 'role:org:admin';

-- enable the not null constraint
ALTER TABLE "memberships" ALTER COLUMN "role" SET NOT NULL;