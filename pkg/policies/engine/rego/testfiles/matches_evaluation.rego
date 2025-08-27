package test_matches_evaluation

matches_evaluation := result {
    # Check if the evaluation contains violations 
    count(input.evaluation_result.violations) > 0
    
    # Check if we have the expected parameter
    input.args.severity == "high"
    
    # If both conditions are met, return true
    result := true
}