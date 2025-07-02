-- Modify "groups" table
ALTER TABLE "groups" ADD COLUMN "member_count" bigint NOT NULL DEFAULT 0;
