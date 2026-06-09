-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "skip_runner_env_vars" boolean NOT NULL DEFAULT false;
