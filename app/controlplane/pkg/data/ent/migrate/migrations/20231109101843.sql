-- Modify "referrers" table
ALTER TABLE "referrers" DROP COLUMN "artifact_type", ADD COLUMN "kind" character varying NOT NULL;
-- Create index "referrer_digest_kind" to table: "referrers"
CREATE UNIQUE INDEX "referrer_digest_kind" ON "referrers" ("digest", "kind");
