-- Modify "cas_mappings" table
ALTER TABLE "cas_mappings" ADD COLUMN "project_id" uuid NULL, ADD COLUMN "cas_mapping_project" uuid NULL;
