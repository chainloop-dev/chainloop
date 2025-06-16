-- update existing records to set the polymorphic values
UPDATE memberships SET
   member_id = user_memberships,
   resource_id = organization_memberships,
   membership_type = 'user',
   resource_type = 'organization';