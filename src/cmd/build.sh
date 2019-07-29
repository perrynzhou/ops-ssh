#!/usr/bin/env bash
export GOPATH=~/Source/perrynzhou/go/devops-foundation/ops-ssh
rm  -rf ./bin/vsh*
rm  -rf ./bin/*.db
rm -rf  ./bin/.vsh_cache.json
rm -rf ./bin/cluster_dump.json
rm -rf ./bin/decode_cluster_dump.json
go build  -o bin/vsh_server     server/app.go
go build  -o bin/vsh     client/app.go
