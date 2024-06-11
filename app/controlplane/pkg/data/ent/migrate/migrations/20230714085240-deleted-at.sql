-- Modify "cas_backends" table
ALTER TABLE "cas_backends" ADD COLUMN "deleted_at" timestamptz NULL;
