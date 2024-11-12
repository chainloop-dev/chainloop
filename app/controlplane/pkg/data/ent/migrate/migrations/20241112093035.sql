-- Modify "workflows" table
ALTER TABLE "workflows" ADD COLUMN "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, ADD COLUMN "latest_run" uuid NULL, ADD CONSTRAINT "workflows_workflow_runs_latest_workflow_run" FOREIGN KEY ("latest_run") REFERENCES "workflow_runs" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;

-- This query modifies all existing Workflows to have the latest_run set to the most recent run
WITH ranked_runs AS (
    SELECT wr.*,
           ROW_NUMBER() OVER (PARTITION BY wr.workflow_workflowruns ORDER BY wr.created_at DESC) AS run_rank
    FROM workflow_runs wr
             JOIN workflows w ON wr.workflow_workflowruns = w.id
)
UPDATE workflows w
SET latest_run = rr.id,
    updated_at = CURRENT_TIMESTAMP
FROM ranked_runs rr
WHERE rr.run_rank = 1
  AND w.id = rr.workflow_workflowruns;
