#! /usr/bin/env bash

set -e

# Run all tests in package
go test -race ./...

# Run only passing netconn.TestConn tests
TEST_CONN=1 go test -race -run "TestConn/(BasicIO|PingPong)"
