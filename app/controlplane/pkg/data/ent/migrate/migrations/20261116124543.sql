-- atlas:txmode none

-- Fill in the new columns of the existing references, in one pass over the table. Outside a
-- transaction with the schema change above, the update holds only row locks, so reads and new
-- references are not blocked while it runs.
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
), "reference_orgs" AS (
  SELECT "rr"."referrer_id", "rr"."referred_by_id", (array_agg("a"."organization_id"))[1] AS "organization_id"
  FROM "referrer_references" AS "rr"
  JOIN "referrer_orgs" AS "a" ON "a"."referrer_id" = "rr"."referrer_id"
  JOIN "referrer_orgs" AS "b" ON "b"."referrer_id" = "rr"."referred_by_id" AND "b"."organization_id" = "a"."organization_id"
  GROUP BY "rr"."referrer_id", "rr"."referred_by_id"
  HAVING count(*) = 1
), "backfill" AS (
  SELECT "rr"."referrer_id", "rr"."referred_by_id", "r"."created_at", "o"."organization_id"
  FROM "referrer_references" AS "rr"
  JOIN "referrers" AS "r" ON "r"."id" = "rr"."referred_by_id"
  LEFT JOIN "reference_orgs" AS "o" ON "o"."referrer_id" = "rr"."referrer_id" AND "o"."referred_by_id" = "rr"."referred_by_id"
)
UPDATE "referrer_references" AS "rr"
SET "created_at" = "b"."created_at", "organization_id" = "b"."organization_id"
FROM "backfill" AS "b"
WHERE "b"."referrer_id" = "rr"."referrer_id" AND "b"."referred_by_id" = "rr"."referred_by_id"
  AND ("rr"."created_at" <> "b"."created_at" OR "rr"."organization_id" IS DISTINCT FROM "b"."organization_id");
ANALYZE "referrer_references";

-- Create index "referrerreference_referrer_id_organization_id_created_at_referred_by_id" to table: "referrer_references"
CREATE INDEX CONCURRENTLY "referrerreference_referrer_id_organization_id_created_at_referred_by_id" ON "referrer_references" ("referrer_id", "organization_id", "created_at" DESC, "referred_by_id" DESC);
