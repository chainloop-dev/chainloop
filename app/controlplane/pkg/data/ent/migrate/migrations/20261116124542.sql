-- Modify "referrer_references" table
-- The foreign keys get the names of the new edge schema. A rename keeps them valid without
-- checking every row again.
ALTER TABLE "referrer_references" RENAME CONSTRAINT "referrer_references_referred_by_id" TO "referrer_references_referrers_referred_by";
ALTER TABLE "referrer_references" RENAME CONSTRAINT "referrer_references_referrer_id" TO "referrer_references_referrers_referrer";
-- A non-volatile default does not rewrite the table, so the lock is held only briefly.
ALTER TABLE "referrer_references" ADD COLUMN "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;
