-- Create "referrers" table
CREATE TABLE "referrers" ("id" uuid NOT NULL, "digest" character varying NOT NULL, "artifact_type" character varying NOT NULL, "downloadable" boolean NOT NULL, "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY ("id"));
-- Create index "referrer_digest_artifact_type" to table: "referrers"
CREATE UNIQUE INDEX "referrer_digest_artifact_type" ON "referrers" ("digest", "artifact_type");
-- Create "referrer_references" table
CREATE TABLE "referrer_references" ("referrer_id" uuid NOT NULL, "referred_by_id" uuid NOT NULL, PRIMARY KEY ("referrer_id", "referred_by_id"), CONSTRAINT "referrer_references_referred_by_id" FOREIGN KEY ("referred_by_id") REFERENCES "referrers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "referrer_references_referrer_id" FOREIGN KEY ("referrer_id") REFERENCES "referrers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
