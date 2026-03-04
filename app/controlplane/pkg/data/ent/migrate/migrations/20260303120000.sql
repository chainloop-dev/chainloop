-- Add API token management policies to existing org-level tokens.
UPDATE "api_tokens"
SET "policies" = "policies" || '[
  {"Resource": "api_token", "Action": "create"},
  {"Resource": "api_token", "Action": "list"},
  {"Resource": "api_token", "Action": "delete"}
]'::jsonb
WHERE "policies" IS NOT NULL
  AND "organization_id" IS NOT NULL
  AND "project_id" IS NULL;
