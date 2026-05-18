-- atlas:txmode none

-- Denormalize organization_id onto workflow_runs so org-scoped list/aggregate
-- queries become sargable without joining workflows.
--
-- Why a trigger?
--
-- The control plane deploys as a multi-replica Deployment with rolling
-- updates. When this migration runs (in the initContainer of a new pod),
-- old pods are still serving traffic with code that does NOT set
-- organization_id on INSERT. The moment step 6 below enforces NOT NULL,
-- every INSERT from those old pods would fail with a constraint violation
-- until the rolling update replaces them — a window of seconds to minutes
-- in which workflow run creation is broken org-wide.
--
-- The BEFORE INSERT trigger below bridges that window: whenever a writer
-- doesn't set organization_id, the trigger fills it from the parent
-- workflow via a single PK lookup (~0.1ms). New code paths set the column
-- explicitly so the trigger's IF check short-circuits; the trigger only
-- does real work for inserts coming from the old replicas. Once every
-- replica runs the new code, the trigger is dead weight — a follow-up
-- release will drop both the trigger and its function.

-- 1. Nullable add (catalog-only, instant).
ALTER TABLE "workflow_runs" ADD COLUMN "organization_id" uuid;

-- 2. FK NOT VALID (no scan, brief AccessExclusive lock).
ALTER TABLE "workflow_runs"
    ADD CONSTRAINT "workflow_runs_organizations_workflowruns"
    FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id")
    ON UPDATE NO ACTION ON DELETE CASCADE NOT VALID;

-- 3. Trigger function: fills organization_id from the parent workflow when
-- the caller didn't set it. Removed by a follow-up migration in the next
-- release once all replicas set the column explicitly.
CREATE OR REPLACE FUNCTION fill_workflow_run_organization_id() RETURNS trigger AS $$
BEGIN
    IF NEW.organization_id IS NULL THEN
        SELECT organization_id INTO NEW.organization_id
        FROM "workflows" WHERE id = NEW.workflow_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER workflow_runs_fill_organization_id
    BEFORE INSERT ON "workflow_runs"
    FOR EACH ROW EXECUTE FUNCTION fill_workflow_run_organization_id();

-- 4. Batched backfill. Concurrent inserts from old replicas are protected by
-- the trigger above, so they can't introduce new NULL rows mid-loop.
-- One COMMIT per batch keeps the longest row-lock window in the millisecond
-- range and avoids one giant WAL entry.
DO $$
DECLARE
    rows_done INT;
BEGIN
    LOOP
        WITH batch AS (
            SELECT wr.id, w.organization_id
            FROM "workflow_runs" wr
            JOIN "workflows" w ON wr.workflow_id = w.id
            WHERE wr.organization_id IS NULL
            LIMIT 5000
        )
        UPDATE "workflow_runs" wr
        SET organization_id = b.organization_id
        FROM batch b
        WHERE wr.id = b.id;
        GET DIAGNOSTICS rows_done = ROW_COUNT;
        COMMIT;
        EXIT WHEN rows_done = 0;
    END LOOP;
END $$;

-- 5. Validate the FK now that data is consistent. SHARE UPDATE EXCLUSIVE
-- permits concurrent DML.
ALTER TABLE "workflow_runs" VALIDATE CONSTRAINT "workflow_runs_organizations_workflowruns";

-- 6. Enforce NOT NULL. In PG 12+ this is a verify-only scan (no rewrite).
-- Safe because the trigger guarantees no concurrent NULL inserts.
ALTER TABLE "workflow_runs" ALTER COLUMN "organization_id" SET NOT NULL;

-- 7. Create the org-scoped list index without blocking writes.
CREATE INDEX CONCURRENTLY "workflowrun_organization_id_created_at"
    ON "workflow_runs" ("organization_id", "created_at" DESC);
