package source_commit

import rego.v1

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
