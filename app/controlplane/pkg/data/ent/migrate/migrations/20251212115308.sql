-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "restrict_contract_creation_to_org_admins" boolean NOT NULL DEFAULT false;
