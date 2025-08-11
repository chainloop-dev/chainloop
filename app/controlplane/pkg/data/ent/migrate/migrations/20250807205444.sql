-- Modify "projects" table
ALTER TABLE "projects" ADD COLUMN "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;
UPDATE "projects" SET "updated_at" = "created_at";
