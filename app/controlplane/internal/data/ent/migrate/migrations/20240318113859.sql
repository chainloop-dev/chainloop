-- Modify "robot_accounts" table
ALTER TABLE "robot_accounts" ADD COLUMN "organization_id" uuid;

-- update organization id for robot accounts
UPDATE "robot_accounts" 
SET "organization_id" = (
    SELECT "organization_id" FROM "workflows" WHERE "workflows"."id" = "robot_accounts"."workflow_robotaccounts"
);

ALTER TABLE "robot_accounts" ALTER COLUMN "organization_id" SET NOT NULL;