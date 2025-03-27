-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "has_restricted_access" boolean NULL;
-- Create index "user_has_restricted_access" to table: "users"
CREATE INDEX "user_has_restricted_access" ON "users" ("has_restricted_access");
