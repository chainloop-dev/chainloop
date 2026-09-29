-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD CONSTRAINT "apitoken_project_ids_only_for_product" CHECK (((project_ids IS NOT NULL) = (NOT ((scope)::text IS DISTINCT FROM 'product'::text))) AND ((project_ids IS NULL) OR (jsonb_typeof(project_ids) = 'array'::text))), ADD COLUMN "project_ids" jsonb NULL;
