-- This query modifies all existing Workflows to have the updated_at field set to the most recent WorkflowRun's created_at field.
WITH ranked_runs AS (
    SELECT wr.*,
           ROW_NUMBER() OVER (PARTITION BY wr.workflow_workflowruns ORDER BY wr.created_at DESC) AS run_rank
    FROM workflow_runs wr
             JOIN workflows w ON wr.workflow_workflowruns = w.id
)
UPDATE workflows w
SET updated_at = rr.created_at
FROM ranked_runs rr
WHERE rr.run_rank = 1
  AND w.id = rr.workflow_workflowruns;
