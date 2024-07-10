package main

deny[msg] {
    not is_workflow

    msg := "incorrect workflow"
}

deny[msg] {
    not is_github

    msg := "incorrect runner"
}


is_workflow {
    input.workflow.name == "policytest"
}

is_github {
    input.runnerType == "GITHUB_ACTION"
}