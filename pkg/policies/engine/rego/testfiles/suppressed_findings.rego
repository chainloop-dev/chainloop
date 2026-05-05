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

# A vulnerability becomes a finding (a real violation) only when it is not
# suppressed. Suppressed entries surface in `suppressed_findings` instead and
# do NOT appear in `findings`.
findings contains v if {
	some vuln in input.vulnerabilities
	not vuln.suppressed
	v := {
		"message": sprintf("Found vulnerability %s", [vuln.id]),
		"external_id": vuln.id,
		"package_purl": vuln.purl,
		"severity": vuln.severity,
	}
}

# Suppressed findings: same shape as a finding, plus the chainloop_*
# correlation fields. Read with object.get so that omitted optional fields
# fall back to safe zero values instead of making the whole entry undefined.
suppressed_findings contains v if {
	some vuln in input.vulnerabilities
	vuln.suppressed == true
	v := {
		"message": sprintf("Found vulnerability %s", [vuln.id]),
		"external_id": vuln.id,
		"package_purl": vuln.purl,
		"severity": vuln.severity,
		"chainloop_finding_id": object.get(vuln, "chainloop_finding_id", ""),
		"chainloop_assessment_ids": object.get(vuln, "chainloop_assessment_ids", []),
	}
}
