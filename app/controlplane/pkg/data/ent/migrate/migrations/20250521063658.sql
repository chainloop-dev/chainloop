-- Modify "project_versions" table
-- 1. Add column as nullable first (non-blocking)
ALTER TABLE "project_versions" ADD COLUMN "updated_at" timestamptz DEFAULT CURRENT_TIMESTAMP;

-- 2. Update existing records to have updated_at = created_at (in batches to avoid blocking)
DO $$
DECLARE
    batch_size INTEGER := 1000;
    total_remaining INTEGER := 0;
BEGIN
    -- Get initial count of rows that need updating
    SELECT COUNT(*) INTO total_remaining FROM "project_versions" WHERE "updated_at" IS NULL;

    -- Loop until no more rows remain
    WHILE total_remaining > 0 LOOP
            WITH cte AS (
                SELECT id
                FROM "project_versions"
                WHERE "updated_at" IS NULL
                LIMIT batch_size
            )
            UPDATE "project_versions"
            SET "updated_at" = "created_at"
            FROM cte
            WHERE "project_versions".id = cte.id;

            -- Update the remaining count
            SELECT COUNT(*) INTO total_remaining
            FROM "project_versions"
            WHERE "updated_at" IS NULL;
        END LOOP;
END $$;

-- 3. Make column NOT NULL after populating data
ALTER TABLE "project_versions" ALTER COLUMN "updated_at" SET NOT NULL;
