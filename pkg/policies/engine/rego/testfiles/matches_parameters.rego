package test_matches_parameters

severity_levels := ["low", "medium", "high", "critical"]

severity_index(level) := index {
    some i
    severity_levels[i] == level
    index := i
}

matches_parameters := result {
    eval_severity := input.args.severity
    expected_severity := input.expected_args.severity
    
    eval_idx := severity_index(eval_severity)
    expected_idx := severity_index(expected_severity)
    
    # Expected severity must be >= evaluation severity
    result := expected_idx >= eval_idx
}