#!/bin/bash
go clean; go build
CATTLE_URL=http://localhost:8080/v1 CATTLE_URL_FOR_AGENT=http://10.0.2.2:8080 ./go-machine-service
