-- Modify "cas_backends" table
ALTER TABLE "cas_backends" ADD COLUMN "managed" boolean NOT NULL DEFAULT false;
