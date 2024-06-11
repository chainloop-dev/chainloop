-- Modify "api_tokens" table
ALTER TABLE "api_tokens" DROP CONSTRAINT "api_tokens_organizations_organization", ADD CONSTRAINT "api_tokens_organizations_api_tokens" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
