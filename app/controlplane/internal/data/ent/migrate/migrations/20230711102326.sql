-- Modify "cas_backends" table
-- Rename repo => name
ALTER TABLE "cas_backends" RENAME COLUMN "repo" TO "name";
-- Update foreign key references
ALTER TABLE "cas_backends" RENAME COLUMN "organization_oci_repositories" TO "organization_cas_backends";
ALTER TABLE "cas_backends" DROP CONSTRAINT "oci_repositories_organizations_oci_repositories";
ALTER TABLE "cas_backends" ADD CONSTRAINT "cas_backends_organizations_cas_backends" FOREIGN KEY ("organization_cas_backends") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
