-- Modify "cas_backends" table
ALTER TABLE "cas_backends" ADD COLUMN "description" character varying NULL;
ALTER TABLE "cas_backends" RENAME COLUMN "name" TO "location";