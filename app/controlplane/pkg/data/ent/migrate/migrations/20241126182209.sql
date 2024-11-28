-- remove for performance reasons
ALTER TABLE cas_mappings DROP CONSTRAINT cas_mappings_cas_backends_cas_backend;
ALTER TABLE cas_mappings DROP CONSTRAINT cas_mappings_organizations_organization;