package main
violations[msg] {
  not is_workflow
  msg := "incorrect workflow"
}

is_workflow {
  input.workflow.name == "policytest"
}
