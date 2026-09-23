-- Modify "api_tokens" table: product is the only scope kind stored
ALTER TABLE "api_tokens" ADD CONSTRAINT "apitoken_scope_product_only" CHECK ((scope IS NULL) OR ((scope)::text = 'product'::text));
