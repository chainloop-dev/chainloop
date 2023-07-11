-- Add provider and set existing to OCI type
ALTER TABLE "oci_repositories" ADD COLUMN "provider" character varying NULL;
UPDATE "oci_repositories" SET provider = 'OCI';
ALTER TABLE "oci_repositories" ALTER COLUMN provider SET NOT NULL;

-- Create "rename table" table
ALTER TABLE "oci_repositories" RENAME TO "cas_backends";
ALTER INDEX "oci_repositories_pkey" RENAME TO "cas_backends_pkey";

-- Rename repo => name
ALTER TABLE "cas_backends" RENAME COLUMN "repo" TO "name";
-- Update foreign key references
ALTER TABLE "cas_backends" RENAME COLUMN "organization_oci_repositories" TO "organization_cas_backends";
ALTER TABLE "cas_backends" DROP CONSTRAINT "oci_repositories_organizations_oci_repositories";
ALTER TABLE "cas_backends" ADD CONSTRAINT "cas_backends_organizations_cas_backends" FOREIGN KEY ("organization_cas_backends") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
