package main

violations[msg] {
    not is_workflow

    msg := "incorrect workflow"
}

violations[msg] {
    not is_github

    msg := "incorrect runner"
}


is_workflow {
    input.predicate.metadata.name == "chainloop-vault-release"
}

is_github {
    input.predicate.runnerType == "GITHUB_ACTION"
    input.predicate.env.GITHUB_SHA
}