-- Modify "organizations" table
ALTER TABLE "organizations" ADD COLUMN "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;

UPDATE "organizations" SET "updated_at" = "created_at";