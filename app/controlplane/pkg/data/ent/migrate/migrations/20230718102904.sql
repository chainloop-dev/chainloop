-- Modify "cas_backends" table
ALTER TABLE "cas_backends" ADD COLUMN "fallback" boolean NOT NULL DEFAULT false;
