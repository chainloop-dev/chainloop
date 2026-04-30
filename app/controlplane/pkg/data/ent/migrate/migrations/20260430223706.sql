-- atlas:txmode none

-- One-shot cleanup of orphaned memberships.
-- memberships.member_id is polymorphic and has no FK to users/groups, so deletes
-- that bypass the app-level cascade leave project/product membership rows pointing
-- at vanished users or groups.
DELETE FROM "memberships"
WHERE "membership_type" = 'user'
  AND NOT EXISTS (SELECT 1 FROM "users" u WHERE u."id" = "memberships"."member_id");

DELETE FROM "memberships"
WHERE "membership_type" = 'group'
  AND NOT EXISTS (SELECT 1 FROM "groups" g WHERE g."id" = "memberships"."member_id");
