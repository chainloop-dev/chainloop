-- Create "login_handoffs" table
CREATE TABLE "login_handoffs" (
  "id" uuid NOT NULL,
  "code_hash" character varying NOT NULL,
  "token_ttl_seconds" bigint NOT NULL,
  "code_challenge" character varying NOT NULL,
  "callback" character varying NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expires_at" timestamptz NOT NULL,
  "used_at" timestamptz NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "login_handoffs_users_user" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "login_handoffs_code_hash_key" to table: "login_handoffs"
CREATE UNIQUE INDEX "login_handoffs_code_hash_key" ON "login_handoffs" ("code_hash");
-- Create index "loginhandoff_expires_at" to table: "login_handoffs"
CREATE INDEX "loginhandoff_expires_at" ON "login_handoffs" ("expires_at");
