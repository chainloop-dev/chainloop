-- atlas:txmode none

-- Existing references take the creation time of the referrer they point to. Outside a transaction
-- with the schema change above, the update holds only row locks, so reads and new references are
-- not blocked while it runs.
UPDATE "referrer_references" AS "rr" SET "created_at" = "r"."created_at" FROM "referrers" AS "r" WHERE "r"."id" = "rr"."referred_by_id" AND "rr"."created_at" <> "r"."created_at";
ANALYZE "referrer_references";

-- Create index "referrerreference_referrer_id_created_at_referred_by_id" to table: "referrer_references"
CREATE INDEX CONCURRENTLY IF NOT EXISTS "referrerreference_referrer_id_created_at_referred_by_id" ON "referrer_references" ("referrer_id", "created_at" DESC, "referred_by_id" DESC);
