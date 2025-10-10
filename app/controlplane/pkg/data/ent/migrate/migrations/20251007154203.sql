-- Backfill released_at field for released project versions
-- This migration populates the released_at timestamp for project versions
-- that are marked as released (prerelease = false) but don't have a released_at timestamp

DO $$
DECLARE
    batch_size INTEGER := 100;
    total_remaining INTEGER := 0;
    total_updated INTEGER := 0;
BEGIN
    -- Get initial count of project versions that need updating
    -- (those that are released but don't have released_at set)
    SELECT COUNT(*)
    INTO total_remaining
    FROM project_versions
    WHERE prerelease = false
      AND released_at IS NULL
      AND deleted_at IS NULL;

    -- Loop until no more rows remain
    WHILE total_remaining > 0 LOOP
        -- Update a batch of project versions
        -- Set released_at to created_at as a reasonable default
            WITH updated AS (
                UPDATE project_versions
                    SET released_at = created_at
                    WHERE id IN (
                        SELECT id
                        FROM project_versions
                        WHERE prerelease = false
                          AND released_at IS NULL
                          AND deleted_at IS NULL
                        LIMIT batch_size
                    )
                    RETURNING id
            )
            SELECT COUNT(*) INTO batch_size FROM updated;

            total_updated := total_updated + batch_size;

            -- Update the remaining count
            SELECT COUNT(*)
            INTO total_remaining
            FROM project_versions
            WHERE prerelease = false
              AND released_at IS NULL
              AND deleted_at IS NULL;

        END LOOP;

END $$;
