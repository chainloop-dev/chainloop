-- Fix group member counts by calculating the correct number of members for each group
-- This migration addresses any inconsistencies in the member_count field of the groups table
-- by counting the actual number of non-deleted memberships for each group.

-- Update the member_count in groups table based on a count of active memberships
UPDATE "groups" g
SET "member_count" = (
    SELECT COUNT(*)
    FROM "group_memberships" gm
    WHERE gm."group_id" = g."id"
      AND gm."deleted_at" IS NULL
)
WHERE 1=1; -- Apply to all groups
