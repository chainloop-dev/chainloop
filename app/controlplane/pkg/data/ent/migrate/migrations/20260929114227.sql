-- atlas:txmode none

-- Create index "apitoken_scope_id" to table: "api_tokens"
CREATE INDEX CONCURRENTLY "apitoken_scope_id" ON "api_tokens" ("scope_id") WHERE ((scope)::text = 'product'::text) AND (revoked_at IS NULL);
