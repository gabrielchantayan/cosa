#!/bin/bash

echo "Building cosa"

echo "Stopping cosa"
cosa stop

echo "Cleaning up old binaries"

rm -rf bin/
rm -f ~/.cosa/cosa.sock
rm -f ~/.cosa/cosad.pid

echo "Building new binaries"

go build -o bin/cosa ./cmd/cosa
go build -o bin/cosad ./cmd/cosad

echo "Starting cosa"
cosa start

echo "Done"
