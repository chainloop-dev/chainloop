-- Updates the contract_revision_used field in workflow_runs to the revision of the contract version used in the run
-- for existing values in the database if it is not set (0)
UPDATE workflow_runs
SET contract_revision_used = wcv.revision
FROM workflow_contract_versions wcv
WHERE workflow_runs.contract_revision_used = 0
AND workflow_runs.workflow_run_contract_version = wcv.id;