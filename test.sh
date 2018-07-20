#! /usr/bin/env bash

set -e

COVER_FLAGS=""
if [ -v COVER ]
then
    echo "Running tests with coverage"
    COVER_FLAGS="-coverprofile=/tmp/coverage.txt -covermode=atomic"
fi

function append-coverage {
    if [ -f /tmp/coverage.txt ]
    then
        cat /tmp/coverage.txt >> coverage.txt
    fi
}

rm -f coverage.txt /tmp/coverage.txt

# Run all tests in package
go test -race ${COVER_FLAGS} ./...
append-coverage

# Run only passing netconn.TestConn tests
TEST_CONN=1 go test -race -run "TestConn/(BasicIO|PingPong)" ${COVER_FLAGS}
append-coverage
