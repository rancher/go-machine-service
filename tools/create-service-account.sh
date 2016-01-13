#!/bin/bash

# See if it exists:
ID=$(curl  -X GET -H 'Accept: application/json' -H 'Content-Type: application/json' 'http://localhost:8080/v1/accounts?uuid=service' | jq -r '.data[0].id')
# Create the account
[ "null" == ${ID} ] && ID=$(curl  -X POST -H 'Accept: application/json' -H 'Content-Type: application/json' -d '{"kind":"service", "name":"service", "uuid":"service"}' 'http://localhost:8080/v1/accounts' | jq -r '.id')

# Create api keys for the account
curl  -X POST -H 'Accept: application/json' -H 'Content-Type: application/json' -d "{\"accountId\":\"$ID\", \"name\":\"service\", \"publicValue\":\"service\", \"secretValue\":\"servicepass\"}" 'http://localhost:8080/v1/apikeys'
