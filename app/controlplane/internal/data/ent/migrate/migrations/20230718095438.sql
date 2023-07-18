-- Modify "cas_backends" table
ALTER TABLE "cas_backends" ADD COLUMN "inline" boolean NOT NULL DEFAULT false;
