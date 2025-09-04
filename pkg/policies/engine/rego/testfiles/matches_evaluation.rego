package test_matches_evaluation

default matches_evaluation := true

matches_evaluation := false if {
    # Check if the evaluation contains violations 
    count(input.violations) > 0
    
    # Check if we have the expected parameter
    input.expected_args.severity == "high"
}