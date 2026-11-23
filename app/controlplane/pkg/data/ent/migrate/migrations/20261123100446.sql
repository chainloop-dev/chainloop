-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "env_var_redaction" jsonb NULL;
