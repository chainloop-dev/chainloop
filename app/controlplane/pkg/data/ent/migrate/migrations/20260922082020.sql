-- Modify "api_tokens" table: scope and scope_id are set together or not at all, and never
-- alongside project_id
ALTER TABLE "api_tokens" ADD CONSTRAINT "apitoken_scope_all_or_nothing" CHECK ((scope IS NULL) = (scope_id IS NULL)), ADD CONSTRAINT "apitoken_scope_excludes_project" CHECK ((project_id IS NULL) OR (scope_id IS NULL));
