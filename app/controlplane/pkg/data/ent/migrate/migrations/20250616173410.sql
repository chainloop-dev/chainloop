-- Modify "memberships" table
ALTER TABLE "memberships" ADD COLUMN "membership_type" character varying NOT NULL, ADD COLUMN "member_id" uuid NOT NULL, ADD COLUMN "resource_type" character varying NOT NULL, ADD COLUMN "resource_id" uuid NOT NULL;
-- Create index "membership_membership_type_member_id_resource_type_resource_id" to table: "memberships"
CREATE UNIQUE INDEX "membership_membership_type_member_id_resource_type_resource_id" ON "memberships" ("membership_type", "member_id", "resource_type", "resource_id");
