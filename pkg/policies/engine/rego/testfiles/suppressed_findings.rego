package main

import rego.v1

################################
# Common section do NOT change #
################################

result := {
	"skipped": skipped,
	"findings": findings,
	"suppressed_findings": suppressed_findings,
	"skip_reason": skip_reason,
}

default skip_reason := ""

skip_reason := m if {
	not valid_input
	m := "invalid input"
}

default skipped := true

skipped := false if valid_input

########################################
# EO Common section, custom code below #
########################################

valid_input := true

findings contains v if {
	some vuln in input.vulnerabilities
	v := {
		"message": sprintf("Found vulnerability %s", [vuln.id]),
		"external_id": vuln.id,
		"package_purl": vuln.purl,
		"severity": vuln.severity,
	}
}

# A finding is suppressed when the input marks it as suppressed; same shape as
# the corresponding entry in `findings`, plus the chainloop_* correlation fields.
suppressed_findings contains v if {
	some vuln in input.vulnerabilities
	vuln.suppressed == true
	v := {
		"message": sprintf("Found vulnerability %s", [vuln.id]),
		"external_id": vuln.id,
		"package_purl": vuln.purl,
		"severity": vuln.severity,
		"chainloop_finding_id": vuln.chainloop_finding_id,
		"chainloop_assessment_ids": vuln.chainloop_assessment_ids,
	}
}
