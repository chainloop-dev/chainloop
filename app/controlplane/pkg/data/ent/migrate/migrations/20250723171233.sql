-- Drop index "membership_membership_type_member_id_resource_type_resource_id" from table: "memberships"
DROP INDEX "membership_membership_type_member_id_resource_type_resource_id";
-- Create index "membership_membership_type_mem_33e9cb590a3adfa25d916afabf657740" to table: "memberships"
CREATE UNIQUE INDEX "membership_membership_type_mem_33e9cb590a3adfa25d916afabf657740" ON "memberships" ("membership_type", "member_id", "resource_type", "resource_id", "role", "parent_id") WHERE (parent_id IS NOT NULL);
-- Create index "membership_membership_type_mem_8014883ac7acffee8425ce171cf6f4cf" to table: "memberships"
CREATE UNIQUE INDEX "membership_membership_type_mem_8014883ac7acffee8425ce171cf6f4cf" ON "memberships" ("membership_type", "member_id", "resource_type", "resource_id", "role") WHERE (parent_id IS NULL);
