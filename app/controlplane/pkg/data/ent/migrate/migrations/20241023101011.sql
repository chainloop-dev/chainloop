-- normalize the existing data
INSERT INTO projects (id, name, organization_id) 
    SELECT gen_random_uuid(), project, organization_id FROM workflows GROUP BY project, organization_id;