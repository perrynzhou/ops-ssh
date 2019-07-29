#!/usr/bin/env bash
export GOPATH=~/Source/perrynzhou/go/devops-foundation/ops-ssh
protoc --go_out=plugins=grpc:. service.proto