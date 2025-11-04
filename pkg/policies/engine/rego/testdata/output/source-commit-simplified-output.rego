package source_commit

import rego.v1

result := {
    "skipped": skipped,
    "violations": violations,
    "skip_reason": skip_reason,
}

default skip_reason := ""

skip_reason := m if {
    not valid_input
    m := "the file content is not recognized"
}

default skipped := true

skipped := false if valid_input

default valid_input := true

check_signature if {
	lower(input.args.check_signature) == "true"
}

check_signature if {
	lower(input.args.check_signature) == "yes"
}

violations contains msg if {
	not has_commit
	msg := "missing commit in statement"
}

violations contains msg if {
	has_commit
	check_signature
	not has_signature
	msg := "missing signature in statement commit"
}

has_commit if {
	some sub in input.subject
	sub.name == "git.head"
	sub.digest.sha1
}

has_signature if {
	some sub in input.subject
	sub.name == "git.head"
	sub.annotations.signature
}
