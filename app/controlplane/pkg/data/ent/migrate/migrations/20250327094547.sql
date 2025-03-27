-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "has_restricted_access" boolean NOT NULL DEFAULT true;
-- Create index "user_has_restricted_access" to table: "users"
CREATE INDEX "user_has_restricted_access" ON "users" ("has_restricted_access") WHERE (has_restricted_access IS TRUE);
