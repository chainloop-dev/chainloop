#!/bin/sh
#
# Copyright 2025 The Chainloop Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

# Start Vault in background
vault server -config=/vault/config/local.hcl &
VAULT_PID=$!

# Wait for Vault to start
sleep 2

export VAULT_ADDR='http://127.0.0.1:8200'

# Check if Vault is initialized
if ! vault status | grep -q "Initialized.*true"; then
    echo "Initializing Vault..."
    vault operator init -key-shares=1 -key-threshold=1 > /vault/file/init.txt
fi

# Unseal Vault
echo "Unsealing Vault..."
UNSEAL_KEY=$(grep "Unseal Key 1:" /vault/file/init.txt | awk '{print $4}')
vault operator unseal $UNSEAL_KEY

# Login with root token to create the dev token
ROOT_TOKEN=$(grep "Initial Root Token:" /vault/file/init.txt | awk '{print $4}')
export VAULT_TOKEN=$ROOT_TOKEN

# Create the 'notasecret' token if it doesn't exist
echo "Ensuring 'notasecret' token exists..."
if ! vault token lookup notasecret > /dev/null 2>&1; then
    echo "Token 'notasecret' not found (or lookup failed), creating it..."
    vault token create -id="notasecret" -policy="root"
else
    echo "Token 'notasecret' already exists."
fi

# Enable KV v2 secrets engine at secret/ if not enabled
if ! vault secrets list | grep -q "^secret/"; then
    echo "Enabling KV v2 secrets engine at secret/..."
    vault secrets enable -path=secret kv-v2
else
    echo "Secrets engine already exists at secret/"
fi


# Keep container running
wait $VAULT_PID
