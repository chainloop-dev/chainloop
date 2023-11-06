-- Create "referrers" table
CREATE TABLE "referrers" ("id" uuid NOT NULL, "digest" character varying NOT NULL, "artifact_type" character varying NOT NULL, "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY ("id"));
-- Create index "referrer_digest" to table: "referrers"
CREATE INDEX "referrer_digest" ON "referrers" ("digest");
-- Create "referrer_references" table
CREATE TABLE "referrer_references" ("referrer_id" uuid NOT NULL, "referred_by_id" uuid NOT NULL, PRIMARY KEY ("referrer_id", "referred_by_id"), CONSTRAINT "referrer_references_referred_by_id" FOREIGN KEY ("referred_by_id") REFERENCES "referrers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "referrer_references_referrer_id" FOREIGN KEY ("referrer_id") REFERENCES "referrers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
