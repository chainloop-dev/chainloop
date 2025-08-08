-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;
UPDATE "users" SET "updated_at" = "created_at";
