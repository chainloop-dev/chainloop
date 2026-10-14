package test_matches_evaluation_findings

import rego.v1

default matches_evaluation := true

# count() raises a strict builtin error when input.findings is null.
matches_evaluation := false if {
	count(input.findings) > 0
	not any_finding_reaches(to_number(input.expected_args.score))
}

any_finding_reaches(expected) if {
	some f in input.findings
	f.score >= expected
}
