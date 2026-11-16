-- atlas:txmode none

-- Fill in the new columns of the existing references, in one pass over the table. Outside a
-- transaction with the schema change above, the update holds row locks only, until it ends. Reads
-- and new references are not blocked. Storing an existing reference again waits for the update.
--
-- created_at is the creation time of the referrer the reference points to.
--
-- organization_id is the organization of the attestation that stored the reference. Storing an
-- attestation links every referrer it touches, both ends of each reference included, to the
-- attestation's workflow, so that organization is one that both ends belong to. When the ends
-- share exactly one organization, it is that one. When they share more, the reference stays NULL,
-- which means that it is read in every organization.
WITH "referrer_orgs" AS (
  SELECT DISTINCT "rw"."referrer_id", "w"."organization_id"
  FROM "referrer_workflows" AS "rw"
  JOIN "workflows" AS "w" ON "w"."id" = "rw"."workflow_id"
), "backfill" AS (
  SELECT "rr"."referrer_id", "rr"."referred_by_id", "r"."created_at",
    CASE WHEN count("b"."organization_id") = 1 THEN (array_agg("b"."organization_id") FILTER (WHERE "b"."organization_id" IS NOT NULL))[1] END AS "organization_id"
  FROM "referrer_references" AS "rr"
  JOIN "referrers" AS "r" ON "r"."id" = "rr"."referred_by_id"
  LEFT JOIN "referrer_orgs" AS "a" ON "a"."referrer_id" = "rr"."referrer_id"
  LEFT JOIN "referrer_orgs" AS "b" ON "b"."referrer_id" = "rr"."referred_by_id" AND "b"."organization_id" = "a"."organization_id"
  GROUP BY "rr"."referrer_id", "rr"."referred_by_id", "r"."created_at"
)
UPDATE "referrer_references" AS "rr"
SET "created_at" = "b"."created_at", "organization_id" = "b"."organization_id"
FROM "backfill" AS "b"
WHERE "b"."referrer_id" = "rr"."referrer_id" AND "b"."referred_by_id" = "rr"."referred_by_id"
  AND ("rr"."created_at" <> "b"."created_at" OR "rr"."organization_id" IS DISTINCT FROM "b"."organization_id");
-- The update leaves a dead version of each row. Reclaim them before the index build reads the table.
VACUUM (ANALYZE) "referrer_references";

-- Create index "referrerreference_referrer_id_organization_id_created_at" to table: "referrer_references"
CREATE INDEX CONCURRENTLY "referrerreference_referrer_id_organization_id_created_at" ON "referrer_references" ("referrer_id", "organization_id", "created_at" DESC, "referred_by_id" DESC);
