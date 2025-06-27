-- Drop index "groupmembership_group_id_user_id" from table: "group_memberships"
DROP INDEX "groupmembership_group_id_user_id";
-- Create index "groupmembership_group_id_user_id" to table: "group_memberships"
CREATE UNIQUE INDEX "groupmembership_group_id_user_id" ON "group_memberships" ("group_id", "user_id") WHERE (deleted_at IS NULL);
