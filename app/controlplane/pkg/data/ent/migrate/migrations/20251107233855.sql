-- Modify "api_tokens" table
ALTER TABLE "api_tokens" ADD COLUMN "policies" jsonb NULL;
-- Populate existing tokens with default policies
UPDATE "api_tokens" SET "policies" = '[
  {"Resource": "workflow_run", "Action": "list"},
  {"Resource": "workflow_run", "Action": "read"},
  {"Resource": "workflow", "Action": "read"},
  {"Resource": "workflow", "Action": "list"},
  {"Resource": "workflow", "Action": "create"},
  {"Resource": "workflow_contract", "Action": "list"},
  {"Resource": "workflow_contract", "Action": "read"},
  {"Resource": "workflow_contract", "Action": "update"},
  {"Resource": "workflow_contract", "Action": "create"},
  {"Resource": "cas_artifact", "Action": "read"},
  {"Resource": "referrer", "Action": "read"},
  {"Resource": "organization", "Action": "read"},
  {"Resource": "robot_account", "Action": "create"},
  {"Resource": "integration_available", "Action": "read"},
  {"Resource": "integration_available", "Action": "list"},
  {"Resource": "integration_registered", "Action": "list"},
  {"Resource": "integration_registered", "Action": "read"},
  {"Resource": "integration_registered", "Action": "create"},
  {"Resource": "integration_attached", "Action": "list"},
  {"Resource": "integration_attached", "Action": "create"},
  {"Resource": "cas_artifact", "Action": "create"}
]'::jsonb WHERE "policies" IS NULL;
