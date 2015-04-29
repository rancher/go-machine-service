#!/bin/bash

# Create the account
ID=$(curl  -X POST -H 'Accept: application/json' -H 'Content-Type: application/json' -H 'X-API-Project-Id: USER' -d '{"kind":"service", "name":"service", "uuid":"service"}' 'http://localhost:8080/v1/accounts' | jq -r '.id')

# Create api keys for the account
curl  -X POST -H 'Accept: application/json' -H 'Content-Type: application/json' -H "X-API-Project-Id: $ID" -d "{\"accountId\":\"$ID\", \"name\":\"service\", \"publicValue\":\"service\", \"secretValue\":\"servicepass\"}" 'http://localhost:8080/v1/apikeys'
