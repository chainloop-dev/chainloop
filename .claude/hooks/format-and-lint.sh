#!/bin/bash

# Read hook input from stdin
input=$(cat)

# Extract file path from the input JSON
file_path=$(echo "$input" | jq -r '.tool_input.file_path // empty')

# Only process Go files
if [[ "$file_path" =~ \.go$ ]]; then
    echo "Running go fmt on $file_path"
    go fmt "$file_path"
fi