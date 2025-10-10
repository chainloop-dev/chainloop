-- Modify "cas_backends" table
ALTER TABLE "cas_backends" ADD COLUMN "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- Backfill updated_at for existing records
-- Use validated_at if not null, otherwise use created_at
UPDATE "cas_backends"
SET "updated_at" = COALESCE("validated_at", "created_at");
