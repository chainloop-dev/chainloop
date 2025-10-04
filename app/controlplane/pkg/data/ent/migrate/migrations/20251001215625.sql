-- Backfill last_run_at field from existing workflow runs
-- This migration populates the last_run_at timestamp for project versions
-- based on the most recent workflow run for each version

DO $$
DECLARE
    batch_size INTEGER := 50;
    total_remaining INTEGER := 0;
BEGIN
    -- Get initial count of project versions that need updating
    -- (those with workflow runs but no last_run_at timestamp)
    SELECT COUNT(DISTINCT pv.id)
    INTO total_remaining
    FROM project_versions pv
    WHERE pv.last_run_at IS NULL
    AND pv.deleted_at IS NULL
    AND EXISTS (
        SELECT 1 FROM workflow_runs wr
        WHERE wr.version_id = pv.id
    );

    -- Log the initial count
    RAISE NOTICE 'Starting backfill of last_run_at for % project versions', total_remaining;

    -- Loop until no more rows remain
    WHILE total_remaining > 0 LOOP
        -- Update a batch of project versions with their latest workflow run timestamp
        WITH versions_to_update AS (
            SELECT pv.id
            FROM project_versions pv
            WHERE pv.last_run_at IS NULL
            AND pv.deleted_at IS NULL
            AND EXISTS (
                SELECT 1 FROM workflow_runs wr
                WHERE wr.version_id = pv.id
            )
            LIMIT batch_size
        ),
        latest_runs AS (
            SELECT
                wr.version_id,
                MAX(COALESCE(wr.finished_at, wr.created_at)) as latest_timestamp
            FROM workflow_runs wr
            INNER JOIN versions_to_update vtu ON wr.version_id = vtu.id
            GROUP BY wr.version_id
        )
        UPDATE project_versions pv
        SET last_run_at = lr.latest_timestamp
        FROM latest_runs lr
        WHERE pv.id = lr.version_id;

        -- Update the remaining count
        SELECT COUNT(DISTINCT pv.id)
        INTO total_remaining
        FROM project_versions pv
        WHERE pv.last_run_at IS NULL
        AND pv.deleted_at IS NULL
        AND EXISTS (
            SELECT 1 FROM workflow_runs wr
            WHERE wr.version_id = pv.id
        );

        -- Log progress
        IF total_remaining > 0 THEN
            RAISE NOTICE 'Remaining project versions to backfill: %', total_remaining;
        END IF;
    END LOOP;

    RAISE NOTICE 'Backfill of last_run_at completed successfully';
END $$;
