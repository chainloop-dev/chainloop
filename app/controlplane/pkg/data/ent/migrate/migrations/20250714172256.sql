-- Fix memberships for deleted groups
-- This migration does the following:
-- 1. Removes entries from the membership table where the member_id is a deleted group or resource_id is a deleted group
-- 2. Marks group memberships with deleted group IDs as deleted (sets deleted_at)
-- 3. Marks as deleted any pending invitations for deleted groups

-- First, delete all memberships where the member is a deleted group or the resource is a deleted group
DELETE FROM "memberships"
WHERE "member_id" IN (
    SELECT "id" FROM "groups"
    WHERE "deleted_at" IS NOT NULL
) OR (
    "resource_id" IN (
        SELECT "id" FROM "groups"
        WHERE "deleted_at" IS NOT NULL
    )
    AND "resource_type" = 'group'
);

-- Next, mark all group_memberships as deleted for deleted groups
-- Only update records that don't already have a deleted_at value
UPDATE "group_memberships"
SET
    "deleted_at" = NOW(),
    "updated_at" = NOW()
WHERE
    "group_id" IN (
        SELECT "id" FROM "groups"
        WHERE "deleted_at" IS NOT NULL
    )
  AND "deleted_at" IS NULL;

-- Finally, ark as deleted any pending invitations for deleted groups
UPDATE "org_invitations"
SET
    "deleted_at" = NOW()
WHERE
    "status" = 'pending'
  AND "deleted_at" IS NULL
  AND "context"::jsonb ? 'group_id_to_join'
  AND "context"::jsonb->>'group_id_to_join' IN (
    SELECT "id"::text FROM "groups"
    WHERE "deleted_at" IS NOT NULL
);
