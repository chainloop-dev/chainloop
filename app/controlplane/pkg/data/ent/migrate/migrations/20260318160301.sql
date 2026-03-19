-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "enable_ai_agent_collector" boolean NOT NULL DEFAULT false;
