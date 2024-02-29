-- Modify "memberships" table
ALTER TABLE "memberships" ADD COLUMN "role" character varying NOT NULL DEFAULT 'role:viewer';
-- Set the existing memberships to the role "role:admin" to not to break compatibility
UPDATE "memberships" SET "role" = 'role:admin';

