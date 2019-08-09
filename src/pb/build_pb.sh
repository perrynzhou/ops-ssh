#!/usr/bin/env bash
export GOPATH=~/home/perrynzhou/Source/perrynzhou/go/ops-ssh
protoc --go_out=plugins=grpc:. service.proto