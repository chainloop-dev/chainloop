-- Drop index "membership_membership_type_mem_33e9cb590a3adfa25d916afabf657740" from table: "memberships"
DROP INDEX "membership_membership_type_mem_33e9cb590a3adfa25d916afabf657740";
-- Drop index "membership_membership_type_mem_8014883ac7acffee8425ce171cf6f4cf" from table: "memberships"
DROP INDEX "membership_membership_type_mem_8014883ac7acffee8425ce171cf6f4cf";
-- Create index "membership_membership_type_mem_69a8fe555e26fd9532f5e3fe38ba2651" to table: "memberships"
CREATE UNIQUE INDEX "membership_membership_type_mem_69a8fe555e26fd9532f5e3fe38ba2651" ON "memberships" ("membership_type", "member_id", "resource_type", "resource_id", "parent_id") WHERE (parent_id IS NOT NULL);
-- Create index "membership_membership_type_member_id_resource_type_resource_id" to table: "memberships"
CREATE UNIQUE INDEX "membership_membership_type_member_id_resource_type_resource_id" ON "memberships" ("membership_type", "member_id", "resource_type", "resource_id") WHERE (parent_id IS NULL);
