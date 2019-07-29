#!/usr/bin/env bash
export GOPATH=~/Users/perrynzhou/Source/perrynzhou/go/devops-foundation/ops-ssh
cd $GOPATH && cd vendor
govendor fetch golang.org/x/net/context
govendor fetch google.golang.org/grpc
govendor fetch golang.org/x/crypto/ssh
govendor fetch golang.org/x/crypto/ssh/terminal
govendor fetch github.com/boltdb/bolt
