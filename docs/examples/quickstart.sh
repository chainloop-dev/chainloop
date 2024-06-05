# Install Chainloop
curl -sfL https://docs.chainloop.dev/install.sh | bash -s

# Initialize a Chainloop workflow
chainloop auth login
chainloop wf create --name mywf --project myproject

# Create a restricted token for further operations
export TOKEN=$(chainloop org api-token create --name test-api-token -o json | jq -r ".[].jwt")

cat "hello chainloop" > hello.txt

# Sign and push attestation to Chainloop
chainloop att init --workflow-name mywf --token $TOKEN
chainloop att add --value hello.txt --token $TOKEN
chainloop att push --token TOKEN