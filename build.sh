#!/bin/bash
set +e
CGO_ENABLED=0 GOOS=linux go build -a -o bin/eve-marketwatch ./cmd/
docker build -t antihax/eve-marketwatch .
