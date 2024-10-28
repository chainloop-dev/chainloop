-- Update the workflow runs with the empty version of their project
UPDATE workflow_runs
SET version_id = (
    SELECT pv.id
    FROM project_versions pv
    JOIN workflows w ON pv.project_id = w.project_id
    WHERE w.id = workflow_runs.workflow_workflowruns
    AND pv.version = ''
);

-- Make version_id column NOT NULL
ALTER TABLE workflow_runs ALTER COLUMN version_id SET NOT NULL;
-- Modify "workflow_runs" table
ALTER TABLE "workflow_runs" DROP CONSTRAINT "workflow_runs_project_versions_runs", ADD CONSTRAINT "workflow_runs_project_versions_runs" FOREIGN KEY ("version_id") REFERENCES "project_versions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;